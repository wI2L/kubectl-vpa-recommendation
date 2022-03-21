package cli

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/muesli/termenv"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/wI2L/kubectl-vpa-recommendation/internal/humanize"
	"github.com/wI2L/kubectl-vpa-recommendation/vpa"
)

const (
	treeElemPrefix     = `├─`
	treeLastElemPrefix = `└─`
	tableUnsetCell     = `-`
)

type sortOrder string

const (
	orderAsc  sortOrder = "asc"
	orderDesc sortOrder = "desc"
)

// String implements the pflag.Value interface.
func (so sortOrder) String() string { return string(so) }

// Type implements the pflag.Value interface.
func (so *sortOrder) Type() string { return "string" }

// Set implements the pflag.Value interface.
func (so *sortOrder) Set(s string) error {
	switch sortOrder(s) {
	case orderAsc, orderDesc:
		*so = sortOrder(s)
		return nil
	default:
		return fmt.Errorf("must be either %q or %q", orderAsc, orderDesc)
	}
}

// tableRow represents a single row of a table.
type tableRow struct {
	Name             string
	Namespace        string
	GVK              schema.GroupVersionKind
	Mode             string
	Target           *vpa.TargetController
	TargetName       string
	TargetGVK        schema.GroupVersionKind
	TargetReplicas   int32
	Requests         vpa.ResourceQuantities
	Recommendations  vpa.ResourceQuantities
	CPUDifference    *float64
	MemoryDifference *float64
	Children         []*tableRow
}

func (tr tableRow) toTableData(flags *Flags, isChild bool) []string {
	rowData := make([]string, 0, 9)

	name := tr.Name
	targetName := tr.TargetName

	if flags.ShowKind && !isChild {
		name = fmt.Sprintf(
			"%s/%s",
			termenv.String(strings.ToLower(tr.GVK.GroupKind().String())).Faint(),
			name,
		)
		targetName = fmt.Sprintf(
			"%s/%s",
			termenv.String(strings.ToLower(tr.TargetGVK.GroupKind().String())).Faint(),
			targetName,
		)
	}
	if flags.ShowNamespace {
		rowData = append(rowData, tr.Namespace)
	}
	rowData = append(rowData, name, tr.Mode, targetName)

	if flags.wide {
		rowData = append(rowData,
			formatQuantity(tr.Requests.CPU), formatQuantity(tr.Recommendations.CPU),
		)
	}
	rowData = append(rowData, formatPercentage(tr.CPUDifference, flags))
	if flags.wide {
		var str string
		d := inf.Dec{}
		d.Round(tr.Recommendations.Memory.AsDec(), 0, inf.RoundUp)
		b := d.UnscaledBig()
		switch tr.Requests.Memory.Format {
		case resource.DecimalSI:
			str = humanize.BigBytes(b, 2)
		case resource.BinarySI:
			str = humanize.BigIBytes(b, 2)
		default:
			str = tr.Recommendations.Memory.String()
		}
		rowData = append(rowData,
			formatQuantity(tr.Requests.Memory),
			str,
		)
	}
	rowData = append(rowData, formatPercentage(tr.MemoryDifference, flags))

	return rowData
}

type (
	table    []*tableRow
	lessFunc func(r1, r2 *tableRow) int
)

// Len implements the sort.Len interface.
func (t table) Len() int { return len(t) }

// Swap implements the sort.Len interface.
func (t table) Swap(i, j int) { t[i], t[j] = t[j], t[i] }

// SortBy sorts the table by one or more columns.
func (t table) SortBy(order sortOrder, cols ...string) {
	mts := multiTableSorter{
		less:  make([]lessFunc, 0, len(cols)),
		order: order,
		table: t,
	}
	for _, c := range cols {
		if fn, ok := columnLessFunc[c]; ok {
			mts.less = append(mts.less, fn)
		}
	}
	sort.Stable(mts)
}

const (
	hdrNamespace     = "Namespace"      // The namespace of the VPA resource
	hdrName          = "Name"           // the [type].name of the VPA resource
	hdrMode          = "Mode"           // the mode of the VPA resource
	hdrTarget        = "Target"         // the [type].name of the target controller
	hdrCPURequest    = "CPU Request"    // the CPU request of the pod
	hdrCPUTarget     = "CPU Target"     // the CPU recommendation target
	hdrCPUDifference = "% CPU Diff"     // the % difference between CPU request/recommendation
	hdrMemRequest    = "Memory Request" // the Memory request of the pod
	hdrMemTarget     = "Memory Target"  // the Memory recommendation target
	hdrMemDifference = "% Memory Diff"  // the % difference between memory request/recommendation
)

// Print writes the table to w.
func (t table) Print(w io.Writer, flags *Flags) error {
	tw := newKubectlTableWriter(w)

	if !flags.NoHeaders {
		var headers []string
		if flags.ShowNamespace {
			headers = append(headers, hdrNamespace)
		}
		headers = append(headers, hdrName, hdrMode, hdrTarget)
		if flags.wide {
			headers = append(headers, hdrCPURequest, hdrCPUTarget)
		}
		headers = append(headers, hdrCPUDifference)
		if flags.wide {
			headers = append(headers, hdrMemRequest, hdrMemTarget)
		}
		headers = append(headers, hdrMemDifference)
		tw.SetHeader(headers)
	}
	for _, row := range t {
		tw.Append(row.toTableData(flags, false))
		for _, childRow := range row.Children {
			tw.Append(childRow.toTableData(flags, true))
		}
	}
	tw.Render()

	if flags.ShowStats {
		_, err := os.Stdout.WriteString("\n")
		if err != nil {
			return err
		}
		return t.printStats(w)
	}
	return nil
}

type tableStatFn func(column func(i int) *resource.Quantity) *resource.Quantity

func (t table) printStats(w io.Writer) error {
	tw := newKubectlTableWriter(w)

	statFuncs := []tableStatFn{
		t.sumQuantities,
		t.meanQuantities,
		t.medianQuantities,
	}
	rows := []struct {
		name           string
		quantityGetter func(i int) *resource.Quantity
		asBytes        bool
	}{
		{"CPU Requests (# cores)", func(i int) *resource.Quantity { return t[i].Requests.CPU }, false},
		{"CPU Recommendations (# cores)", func(i int) *resource.Quantity { return t[i].Recommendations.CPU }, false},
		{"MEM Requests (IEC/SI)", func(i int) *resource.Quantity { return t[i].Requests.Memory }, true},
		{"MEM Recommendations (IEC/SI)", func(i int) *resource.Quantity { return t[i].Recommendations.Memory }, true},
	}
	for _, row := range rows {
		values := make([]string, 0, len(statFuncs))

		for _, fn := range statFuncs {
			scaledQuantity := func(i int) *resource.Quantity {
				q := row.quantityGetter(i)

				// Scale the quantity according to the number
				// of replicas declared in the controller's spec.
				var replicas int64
				n, err := t[i].Target.ReplicasCount()
				if err != nil {
					replicas = 1
				} else {
					replicas = n
				}
				return multiplyQuantity(q, replicas)
			}
			q := fn(scaledQuantity)

			s := tableUnsetCell
			if q != nil {
				if row.asBytes {
					d := inf.Dec{}
					d.Round(q.AsDec(), 0, inf.RoundUp)
					b := d.UnscaledBig()
					s = humanize.BigIBytes(b, 2) + "/" + humanize.BigBytes(b, 2)
					s = strings.ReplaceAll(s, " ", "")
				} else {
					s = q.AsDec().Round(q.AsDec(), 2, inf.RoundCeil).String()
				}
			}
			values = append(values, s)
		}
		tw.Append(append([]string{row.name}, values...))
	}
	tw.SetHeader([]string{"Description", "Total", "Mean", "Median"})
	tw.Render()

	return nil
}

func multiplyQuantity(q *resource.Quantity, n int64) *resource.Quantity {
	if q == nil || n == 0 {
		return nil
	}
	if n == 1 {
		return q
	}
	// The resource.Quantity type does not define a
	// multiplication method, so instead we add the
	// same amount n-1 times.
	ret := q.DeepCopy()
	for i := 0; int64(i) < n-1; i++ {
		ret.Add(*q)
	}
	return &ret
}

func (t table) sumQuantities(column func(i int) *resource.Quantity) *resource.Quantity {
	var sum resource.Quantity
	for i := range t {
		v := column(i)
		if v != nil {
			sum.Add(*v)
		}
	}
	return &sum
}

func (t table) meanQuantities(column func(i int) *resource.Quantity) *resource.Quantity {
	sum := t.sumQuantities(column)
	dec := sum.AsDec()
	tmp := inf.Dec{}
	tmp.QuoRound(dec, inf.NewDec(int64(len(t)), 0), dec.Scale(), inf.RoundDown)

	return resource.NewDecimalQuantity(tmp, resource.DecimalSI)
}

func (t table) medianQuantities(column func(i int) *resource.Quantity) *resource.Quantity {
	var values []*resource.Quantity

	// Collect all values and sort them.
	for i := range t {
		v := column(i)
		if v != nil {
			values = append(values, v)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		b := compareQuantities(values[i], values[j])
		switch b {
		case -1:
			return true
		default:
			return false
		}
	})
	// No math is needed if there are no numbers.
	// For even numbers we add the two middle values
	// and divide by two.
	// For odd numbers we just use the middle value.
	l := len(values)
	if l == 0 {
		return nil
	} else if l%2 == 0 {
		q := values[l/2-1]
		q.Add(*(values[l/2]))
		tmp := inf.Dec{}
		tmp.QuoRound(q.AsDec(), inf.NewDec(2, 0), 2, inf.RoundUp)

		return resource.NewDecimalQuantity(tmp, resource.DecimalSI)
	}
	return values[l/2]
}

// newKubectlTableWriter returns a new table writer that writes
// to w and print according to the Kubectl output format.
func newKubectlTableWriter(w io.Writer) *tablewriter.Table {
	tw := tablewriter.NewWriter(w)

	tw.SetAutoWrapText(false)
	tw.SetAutoFormatHeaders(true)
	tw.SetNoWhiteSpace(true)
	tw.SetHeaderLine(false)
	tw.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	tw.SetAlignment(tablewriter.ALIGN_LEFT)
	tw.SetCenterSeparator("")
	tw.SetColumnSeparator("")
	tw.SetRowSeparator("")
	tw.SetTablePadding("   ")
	tw.SetBorder(false)

	return tw
}

// multiTableSorter implements the sort.Sort interface
// for a table. It can sort using different sets of
// multiple fields in the comparison.
type multiTableSorter struct {
	table
	less  []lessFunc
	order sortOrder
}

// Less implements the sort.Interface for multiTableSorter.
func (mts multiTableSorter) Less(i, j int) bool {
	r1, r2 := mts.table[i], mts.table[j]

	for _, lessFn := range mts.less {
		switch x := lessFn(r1, r2); x {
		case -1: // r1 placed before
			return withOrder(true, mts.order)
		case 1: // r2 placed before
			return withOrder(false, mts.order)
		default:
			continue
		}
	}
	// At this point, all comparisons returned 0
	// to indicate equality, so just return false.
	return false
}

var columnLessFunc = map[string]lessFunc{
	"name":      func(r1, r2 *tableRow) int { return strings.Compare(r1.Name, r2.Name) },
	"namespace": func(r1, r2 *tableRow) int { return strings.Compare(r1.Namespace, r2.Namespace) },
	"target":    func(r1, r2 *tableRow) int { return strings.Compare(r1.TargetName, r2.TargetName) },
	"cpu-diff":  func(r1, r2 *tableRow) int { return compareFloat64(r1.CPUDifference, r2.CPUDifference) },
	"mem-diff":  func(r1, r2 *tableRow) int { return compareFloat64(r1.MemoryDifference, r2.MemoryDifference) },
	"cpu-req":   func(r1, r2 *tableRow) int { return compareQuantities(r1.Requests.CPU, r2.Requests.CPU) },
	"mem-req":   func(r1, r2 *tableRow) int { return compareQuantities(r1.Requests.Memory, r2.Requests.Memory) },
	"cpu-rec":   func(r1, r2 *tableRow) int { return compareQuantities(r1.Recommendations.CPU, r2.Recommendations.CPU) },
	"mem-rec": func(r1, r2 *tableRow) int {
		return compareQuantities(r1.Recommendations.Memory, r2.Recommendations.Memory)
	},
}

func formatPercentage(f *float64, flags *Flags) string {
	if f == nil {
		return tableUnsetCell
	}
	n := fmt.Sprintf("%+.2f", *f)

	if termenv.EnvNoColor() || flags.NoColors {
		return n
	}
	p := termenv.ColorProfile()
	s := termenv.String(n)

	warn, crit := flags.WarningThreshold, flags.CriticalThreshold
	switch {
	case *f > -warn && *f < warn:
		s = s.Foreground(p.Color("#A8CC8C"))
	case (*f >= warn && *f < crit) || (*f <= -warn && *f > -crit):
		s = s.Foreground(p.Color("#DBAB79"))
	default:
		s = s.Foreground(p.Color("#E88388"))
	}
	return s.Bold().String()
}

func formatQuantity(q *resource.Quantity) string {
	if q == nil || q.IsZero() {
		return tableUnsetCell
	}
	return q.String()
}

func compareFloat64(f1, f2 *float64) int {
	switch {
	case f1 == nil && f2 == nil:
		return 0
	case f1 == nil:
		return 1
	case f2 == nil:
		return -1
	case *f1 == *f2:
		return 0
	case *f1 < *f2 || (math.IsNaN(*f1) && !math.IsNaN(*f2)):
		return -1
	}
	return 1
}

func compareQuantities(q1, q2 *resource.Quantity) int {
	switch {
	case q1 == nil && q2 == nil:
		return 0
	case q1 == nil:
		return 1
	case q2 == nil:
		return -1
	}
	return q1.Cmp(*q2)
}

func withOrder(b bool, order sortOrder) bool {
	switch order {
	case orderAsc:
		return b
	case orderDesc:
		return !b
	default:
		return b
	}
}
