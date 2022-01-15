package client

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util"
)

const (
	clientQPS      = 50
	clientBurst    = 100
	discoveryBurst = 300
	discoveryQPS   = 50.0
)

// Flags represents the common configuration flags to
// create a client that interact with a Kubernetes cluster.
type Flags struct {
	*genericclioptions.ConfigFlags
}

// DefaultFlags returns a new set client configuration
// flags with default values.
func DefaultFlags() *Flags {
	return &Flags{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
	}
}

// AddFlags binds the client configuration flags
// to the given flag set.
func (f *Flags) AddFlags(flags *pflag.FlagSet) {
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	f.ConfigFlags.AddFlags(fs)

	// Normalize client flags by removing any dot
	// character at the end of the usage string.
	fs.VisitAll(func(f *pflag.Flag) {
		f.Usage = strings.TrimSuffix(f.Usage, ".")
	})
	flags.AddFlagSet(fs)
}

// RegisterCompletionFunc registers the completion functions
// related to client configuration flags.
// Copied from the official `kubectl` command source:
// https://github.com/kubernetes/kubectl/blob/v0.23.1/pkg/cmd/cmd.go#L471-L492
func (*Flags) RegisterCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	directive := cobra.ShellCompDirectiveNoFileComp

	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"namespace",
		func(_ *cobra.Command, _ []string, tc string) ([]string, cobra.ShellCompDirective) {
			return get.CompGetResource(f, cmd, "namespace", tc), directive
		}))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"context",
		func(_ *cobra.Command, _ []string, tc string) ([]string, cobra.ShellCompDirective) {
			return util.ListContextsInConfig(tc), directive
		}))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster",
		func(_ *cobra.Command, _ []string, tc string) ([]string, cobra.ShellCompDirective) {
			return util.ListClustersInConfig(tc), directive
		}))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"user",
		func(_ *cobra.Command, _ []string, tc string) ([]string, cobra.ShellCompDirective) {
			return util.ListUsersInConfig(tc), directive
		}))
}

// NewClient returns a new clients based on the flags' configuration.
func (f *Flags) NewClient() (Interface, error) {
	f.ConfigFlags = f.
		WithDiscoveryQPS(discoveryQPS).
		WithDiscoveryBurst(discoveryBurst).
		WithDeprecatedPasswordFlag()

	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	config.QPS = clientQPS
	config.Burst = clientBurst

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dis, err := f.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	m, err := f.ToRESTMapper()
	if err != nil {
		return nil, err
	}
	c := &client{
		flags:           f,
		dynamicClient:   dyn,
		discoveryClient: dis,
		mapper:          m,
	}
	return c, nil
}
