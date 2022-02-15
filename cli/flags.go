package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/pflag"

	"github.com/wI2L/kubectl-vpa-recommendation/vpa"
)

const (
	flagAllNamespaces           = "all-namespaces"
	flagAllNamespacesShorthand  = "A"
	flagShowNamespace           = "show-namespace"
	flagShowKind                = "show-kind"
	flagShowKindShorthand       = "k"
	flagShowContainers          = "show-containers"
	flagShowContainersShorthand = "c"
	flagNoColors                = "no-colors"
	flagNoHeaders               = "no-headers"
	flagSortOrder               = "sort-order"
	flagSortColumns             = "sort-columns"
	flagLabelSelector           = "selector"
	flagLabelSelectorShorthand  = "l"
	flagFieldSelector           = "field-selector"
	flagOutput                  = "output"
	flagOutputShorthand         = "o"
	flagRecommendationType      = "recommendation-type"
)

const (
	wideOutput      = "wide"
	splitOutput     = "split"
	splitWideOutput = "split-wide"
)

var (
	defaultSortOrder          = orderAsc
	defaultSortColumns        = []string{"namespace", "name"}
	defaultRecommendationType = vpa.RecommendationTarget
)

// Flags represents the common command flags.
type Flags struct {
	AllNamespaces      bool
	ShowNamespace      bool
	ShowKind           bool
	ShowContainers     bool
	NoColors           bool
	NoHeaders          bool
	SortOrder          sortOrder
	SortColumns        []string
	LabelSelector      string
	FieldSelector      string
	Output             string
	RecommendationType vpa.RecommendationType

	wide  bool
	split bool
}

// DefaultFlags returns default command flags.
func DefaultFlags() *Flags {
	f := &Flags{
		SortOrder:          defaultSortOrder,
		SortColumns:        defaultSortColumns,
		RecommendationType: defaultRecommendationType,
	}
	return f
}

// AddFlags binds the command flags to the given pflag.FlagSet.
func (f *Flags) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&f.AllNamespaces, flagAllNamespaces, flagAllNamespacesShorthand, f.AllNamespaces,
		"List VPA resources in all namespaces")

	flags.BoolVar(&f.ShowNamespace, flagShowNamespace, f.ShowNamespace,
		"Show namespace as the first column")

	flags.BoolVarP(&f.ShowKind, flagShowKind, flagShowKindShorthand, f.ShowKind,
		"List the resource type for the requested object(s) and their target")

	flags.BoolVarP(&f.ShowContainers, flagShowContainers, flagShowContainersShorthand, f.ShowContainers,
		"Display containers recommendations for each VPA resource")

	flags.BoolVar(&f.NoColors, flagNoColors, f.NoColors,
		"Do not use colors to highlight increase/decrease percentage values")

	flags.BoolVar(&f.NoHeaders, flagNoHeaders, f.NoHeaders,
		"Do not print table headers")

	flags.Var(&f.SortOrder, flagSortOrder,
		"The sort order of the table columns. Either 'asc' or 'desc'")

	flags.StringSliceVar(&f.SortColumns, flagSortColumns, f.SortColumns,
		fmt.Sprintf("Comma-separated list of column names for sorting the table. Any of: %s", strings.Join(sortColumnsFlagValues(), ", ")))

	flags.StringVarP(&f.LabelSelector, flagLabelSelector, flagLabelSelectorShorthand, f.LabelSelector,
		"Selector (label query) to filter on, supports '=', '==', and '!=' (e.g. -l key1=value1,key2=value2)")

	flags.StringVar(&f.FieldSelector, flagFieldSelector, f.FieldSelector,
		"Selector (field query) to filter on, supports '=', '==', and '!=' (e.g. --field-selector key1=value1,key2=value2)")

	flags.StringVarP(&f.Output, flagOutput, flagOutputShorthand, f.Output,
		"Output format. One of 'wide', 'slit', 'split-wide'")

	flags.Var(&f.RecommendationType, flagRecommendationType,
		fmt.Sprintf("The type of recommendation to use in comparisons. One of: %s", strings.Join(recommendationTypeFlagValues(), ", ")))
}

// Tidy post-processes the flags.
func (f *Flags) Tidy() {
	switch f.Output {
	case wideOutput:
		f.wide = true
	case splitOutput:
		f.split = true
	case splitWideOutput:
		f.wide = true
		f.split = true
	}
}

func sortColumnsFlagValues() []string {
	keys := make([]string, 0, len(columnLessFunc))
	for k := range columnLessFunc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func recommendationTypeFlagValues() []string {
	v := []string{
		singleQuoted(vpa.RecommendationTarget.String()),
		singleQuoted(vpa.RecommendationLowerBound.String()),
		singleQuoted(vpa.RecommendationUpperBound.String()),
		singleQuoted(vpa.RecommendationUncappedTarget.String()),
	}
	sort.Strings(v)
	return v
}

func singleQuoted(s string) string {
	return fmt.Sprintf("'%s'", s)
}
