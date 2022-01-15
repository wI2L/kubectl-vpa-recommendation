package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// getSortColumnsComps returns the completions
// available for the SortColumn flag.
// TODO(will): add description to completions
func getSortColumnsComps(_ *cobra.Command, _ []string, tc string) ([]string, cobra.ShellCompDirective) {
	names := make([]string, 0, len(columnLessFunc))

	for name := range columnLessFunc {
		names = append(names, name)
	}
	sort.Strings(names)
	return filterStringSliceFlagComps(names, tc), cobra.ShellCompDirectiveDefault
}

// filterStringSliceFlagComps filters out the completions
// already present in the original comma-separated string.
func filterStringSliceFlagComps(allComps []string, tc string) []string {
	current := strings.Split(tc, ",")
	return cmdutil.Difference(allComps, current)
}
