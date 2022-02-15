package cli

import (
	"context"
	// Embed command example.
	_ "embed"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/pointer"

	"github.com/wI2L/kubectl-vpa-recommendation/client"
	"github.com/wI2L/kubectl-vpa-recommendation/internal/version"
	"github.com/wI2L/kubectl-vpa-recommendation/vpa"
)

const (
	vpaPlural      = "verticalpodautoscalers"
	cmdShort       = "Compare VerticalPodAutoscaler recommendations to actual resources requests"
	defaultTimeout = 15 * time.Second
)

//go:embed example.txt
var cmdExample string

// CommandOptions represents the options of the command.
type CommandOptions struct {
	Flags         *Flags
	Client        client.Interface
	ClientFlags   *client.Flags
	Namespace     string
	ResourceNames []string

	genericclioptions.IOStreams
}

// NewCmd returns a new initialized command.
func NewCmd(streams genericclioptions.IOStreams, name string) *cobra.Command {
	opts := CommandOptions{
		Flags:       DefaultFlags(),
		ClientFlags: client.DefaultFlags(),
		IOStreams:   streams,
	}
	f := cmdutil.NewFactory(opts.ClientFlags)

	// Register factory for functions called
	// to autocomplete the client config flags.
	util.SetFactoryForCompletion(f)

	cmd := &cobra.Command{
		Use:                   fmt.Sprintf("%s [NAME...] [options]", name),
		Short:                 cmdShort,
		Long:                  cmdShort,
		Example:               fmt.Sprintf(cmdExample, name),
		Args:                  cobra.ArbitraryArgs,
		Version:               version.Get().Raw(),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		DisableSuggestions:    true,
		DisableAutoGenTag:     true,
		Run:                   opts.Run,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, tc string) ([]string, cobra.ShellCompDirective) {
			comps := get.CompGetResource(f, cmd, vpaPlural, tc)
			return comps, cobra.ShellCompDirectiveNoFileComp
		},
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			opts.Flags.Tidy()
		},
	}
	cmd.SetVersionTemplate("{{printf \"%s\" .Version}}\n")

	// Bind client config and common flags
	// to the command's flag set.
	opts.ClientFlags.AddFlags(cmd.Flags())
	opts.Flags.AddFlags(cmd.Flags())

	// Replace the default flags added by cobra.
	cmd.Flags().Bool("version", false, "Print the command version and exit")
	cmd.Flags().BoolP("help", "h", false, "Print the command help and exit")

	// Register autocompletion functions.
	opts.ClientFlags.RegisterCompletionFunc(cmd, f)
	_ = cmd.RegisterFlagCompletionFunc(flagSortColumns, getSortColumnsComps)

	return templates.Normalize(cmd)
}

// Run is the method called by cobra to run the command.
func (co *CommandOptions) Run(c *cobra.Command, args []string) {
	cmdutil.CheckErr(co.Complete(c, args))
	cmdutil.CheckErr(co.Validate(c, args))
	cmdutil.CheckErr(co.Execute())
}

// Complete finishes the setup of the command options.
func (co *CommandOptions) Complete(_ *cobra.Command, _ []string) error {
	rawConfig := co.ClientFlags.ToRawKubeConfigLoader()

	ns, override, err := rawConfig.Namespace()
	if err != nil {
		return err
	}
	if override {
		klog.V(4).Infof("namespace override: %s", ns)
	}
	co.Namespace = ns
	co.Client, err = co.ClientFlags.NewClient()
	if err != nil {
		return fmt.Errorf("couldn't create client: %w", err)
	}
	return nil
}

// Validate ensure that required options to run the
// command are set and valid.
func (co *CommandOptions) Validate(_ *cobra.Command, args []string) error {
	for i, v := range args {
		if trimmed := strings.TrimSpace(v); trimmed == "" {
			klog.Warningf("arg #%d is empty, skipping", i)
		} else {
			co.ResourceNames = append(co.ResourceNames, trimmed)
		}
	}
	klog.V(4).Infof("namespace: %s", co.Namespace)
	klog.V(4).Infof("all namespaces: %b", co.Flags.AllNamespaces)
	klog.V(4).Infof("resource names: %s", strings.Join(co.ResourceNames, ", "))

	return nil
}

// Execute runs the command.
func (co *CommandOptions) Execute() error {
	ctx := context.Background()

	// Check if the cluster is reachable and
	// has the required CustomResourceDefinition.
	err := co.Client.IsClusterReachable()
	if err != nil {
		return err
	}
	gv := vpav1.SchemeGroupVersion
	ok, err := co.Client.HasGroupVersion(gv)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("group %s not available", gv.String())
	}
	vpas, err := co.Client.ListVPAResources(ctx, client.ListOptions{
		Namespace:      co.Namespace,
		AllNamespaces:  co.Flags.AllNamespaces,
		ResourceNames:  co.ResourceNames,
		LabelSelector:  co.Flags.LabelSelector,
		FieldSelector:  co.Flags.FieldSelector,
		TimeoutSeconds: pointer.Int64(int64(defaultTimeout.Seconds())),
		Limit:          250,
	})
	if err != nil {
		return err
	}
	if len(vpas) == 0 {
		if co.Flags.AllNamespaces {
			fmt.Println("No VPA resources found.")
		} else {
			fmt.Printf("No VPA resources found in %s namespace.\n", co.Namespace)
		}
		return nil
	}
	klog.V(4).Infof("fetched %d VPA(s)", len(vpas))

	var tables []table

	if co.Flags.split {
		// Sort the result list by namespace, and create
		// a table of VPA objects for each.
		sort.SliceStable(vpas, func(i, j int) bool {
			return vpas[i].Namespace < vpas[j].Namespace
		})
		j := 0
		for i := 0; i < len(vpas); i++ {
			if i != 0 && vpas[i].Namespace != vpas[i-1].Namespace || i == len(vpas)-1 {
				table := co.bindRecommendationsAndRequests(vpas[j:i])
				tables = append(tables, table)
				j = i
			}
		}
	} else {
		table := co.bindRecommendationsAndRequests(vpas)
		tables = append(tables, table)
	}
	for i := range tables {
		tables[i].SortBy(co.Flags.SortOrder, co.Flags.SortColumns...)
		if err := tables[i].Print(co.Out, co.Flags); err != nil {
			return err
		}
		if i != len(tables)-1 {
			_, err := os.Stdout.WriteString("\n")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// bindRecommendationsAndRequests returns a table that bind the
// recommendations of the VPA in the list to the actual resource
// requests of their target controller's pods.
func (co *CommandOptions) bindRecommendationsAndRequests(list []*vpav1.VerticalPodAutoscaler) table {
	var table table

	for _, v := range list {
		ref := v.Spec.TargetRef
		if ref == nil {
			klog.V(4).Infof("vpa %s/%s has no target", v.Namespace, v.Name)
			continue
		}
		tc, err := vpa.NewTargetController(co.Client, v.Spec.TargetRef, v.Namespace)
		if err != nil {
			klog.V(4).Infof("couldn't get target for vpa %s/%s", v.Namespace, v.Name)
			continue
		}
		row := newTableRow(v, tc, v.Name, co.Flags.RecommendationType)
		table = append(table, row)

		if v.Status.Recommendation != nil && len(v.Status.Recommendation.ContainerRecommendations) > 1 {
			if !co.Flags.ShowContainers {
				continue
			}
			for i, c := range v.Status.Recommendation.ContainerRecommendations {
				prefix := treeElemPrefix

				if i == len(v.Status.Recommendation.ContainerRecommendations)-1 {
					prefix = treeLastElemPrefix
				}
				rqs := tc.GetContainerRequests(c.ContainerName)
				rcs := vpa.ResourceQuantities{
					CPU:    c.Target.Cpu(),
					Memory: c.Target.Memory(),
				}
				childRow := &tableRow{
					Name:             fmt.Sprintf("%s %s", prefix, c.ContainerName),
					Requests:         rqs,
					Recommendations:  rcs,
					CPUDifference:    vpa.DiffQuantitiesAsPercent(rqs.CPU, rcs.CPU),
					MemoryDifference: vpa.DiffQuantitiesAsPercent(rqs.Memory, rcs.Memory),
				}
				row.Children = append(row.Children, childRow)
			}
		}
	}
	return table
}

func newTableRow(v *vpav1.VerticalPodAutoscaler, tc *vpa.TargetController, name string, rt vpa.RecommendationType) *tableRow {
	rqs := tc.GetRequests()
	rcs := vpa.TotalRecommendations(v, rt)

	row := &tableRow{
		Name:             name,
		Namespace:        v.Namespace,
		GVK:              v.GroupVersionKind(),
		Mode:             updateModeFromSpec(v.Spec.UpdatePolicy),
		TargetName:       tc.Name,
		TargetGVK:        tc.GroupVersionKind,
		Requests:         rqs,
		Recommendations:  rcs,
		CPUDifference:    vpa.DiffQuantitiesAsPercent(rqs.CPU, rcs.CPU),
		MemoryDifference: vpa.DiffQuantitiesAsPercent(rqs.Memory, rcs.Memory),
	}
	return row
}

func updateModeFromSpec(spec *vpav1.PodUpdatePolicy) string {
	if spec != nil && spec.UpdateMode != nil {
		return string(*spec.UpdateMode)
	}
	return tableUnsetCell
}
