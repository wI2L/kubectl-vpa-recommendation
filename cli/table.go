package cli

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/muesli/termenv"
	"github.com/olekukonko/tablewriter"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/wI2L/kubectl-vpa-recommendation/vpa"
)

const (
	treeElemPrefix     = `├─`
	treeLastElemPrefix = `└─`
	tableUnsetCell     = `-`
	wideOutput         = "wide"
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
	TargetName       string
	TargetGVK        schema.GroupVersionKind
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
	rowData = append(rowData,
		name,
		tr.Mode,
		targetName,
	)
	if flags.Output == wideOutput {
		rowData = append(rowData,
			formatQuantity(tr.Requests.CPU),
			formatQuantity(tr.Recommendations.CPU),
		)
	}
	rowData = append(rowData, formatPercentage(tr.CPUDifference, flags.NoColors))
	if flags.Output == wideOutput {
		rowData = append(rowData,
			formatQuantity(tr.Requests.Memory),
			formatQuantity(tr.Recommendations.Memory),
		)
	}
	rowData = append(rowData, formatPercentage(tr.MemoryDifference, flags.NoColors))
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
	tw := tablewriter.NewWriter(w)
	setKubectlTableFormat(tw)

	if !flags.NoHeaders {
		var headers []string
		if flags.ShowNamespace {
			headers = append(headers, hdrNamespace)
		}
		headers = append(headers, hdrName, hdrMode, hdrTarget)
		if flags.Output == wideOutput {
			headers = append(headers, hdrCPURequest, hdrCPUTarget)
		}
		headers = append(headers, hdrCPUDifference)
		if flags.Output == wideOutput {
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

	return nil
}

// setKubectlTableFormat configures the given writer
// to print according to the Kubectl output format.
func setKubectlTableFormat(t *tablewriter.Table) {
	t.SetAutoWrapText(false)
	t.SetAutoFormatHeaders(true)
	t.SetNoWhiteSpace(true)
	t.SetHeaderLine(false)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetCenterSeparator("")
	t.SetColumnSeparator("")
	t.SetRowSeparator("")
	t.SetTablePadding("   ")
	t.SetBorder(false)
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

func formatPercentage(f *float64, noColors bool) string {
	if f == nil {
		return tableUnsetCell
	}
	n := fmt.Sprintf("%+.2f", *f)

	if termenv.EnvNoColor() || noColors {
		return n
	}
	p := termenv.ColorProfile()
	s := termenv.String(n)

	switch {
	case *f >= -10 && *f <= 20:
		s = s.Foreground(p.Color("#A8CC8C"))
	case (*f > 20 && *f < 50) || (*f < -10 && *f > -50):
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
