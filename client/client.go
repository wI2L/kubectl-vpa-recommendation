package client

import (
	"context"
	"fmt"
	"strings"
	"sync"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/pager"
)

const vpaKind = "VerticalPodAutoscaler"

// ListOptions represents the options for listing resources.
type ListOptions struct {
	Namespace      string
	AllNamespaces  bool
	ResourceNames  []string
	FieldSelector  string
	LabelSelector  string
	TimeoutSeconds *int64
	Limit          int64
}

// Interface captures the methods of a client used to
// interact with a Kubernetes cluster.
type Interface interface {
	GetRESTMapper() (meta.RESTMapper, error)
	IsClusterReachable() error
	HasGroupVersion(version schema.GroupVersion) (bool, error)
	ListVPAResources(context.Context, ListOptions) ([]*vpav1.VerticalPodAutoscaler, error)
	GetVPATarget(context.Context, *autoscalingv1.CrossVersionObjectReference, string) (*unstructuredv1.Unstructured, error)
	ListDependentPods(ctx context.Context, targetMeta metav1.ObjectMeta) ([]*corev1.Pod, error)
}

var _ Interface = (*client)(nil)

// client is a concrete implementation of a Kubernetes
// client configured from the common command-line flags.
type client struct {
	flags           *Flags
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	coreClient      *corev1client.CoreV1Client
	mapper          meta.RESTMapper

	// lock during lazy init of the client
	sync.Mutex
}

// GetRESTMapper returns a new REST mapper to map the
// types defined in a runtime.Scheme to REST endpoints.
func (c *client) GetRESTMapper() (meta.RESTMapper, error) {
	c.Lock()
	defer c.Unlock()
	if c.mapper != nil {
		return c.mapper, nil
	}
	mapper, err := c.flags.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("cannot create REST mapper: %w", err)
	}
	return mapper, err
}

// IsClusterReachable tests the connectivity to the remote
// Kubernetes cluster used by the current client.
func (c *client) IsClusterReachable() error {
	_, err := c.discoveryClient.ServerVersion()
	if err != nil {
		reason := apierrors.ReasonForError(err)
		return fmt.Errorf("cluster unavailable (%s): %w", reason, err)
	}
	return nil
}

// HasGroupVersion returns whether the remote Kubernetes
// cluster has the CustomResourceDefinitions of the given
// GroupVersion API.
func (c *client) HasGroupVersion(gv schema.GroupVersion) (bool, error) {
	apiGroups, err := c.discoveryClient.ServerGroups()
	if err != nil {
		return false, fmt.Errorf("couldn't get available api groups/versions from server: %w", err)
	}
	for _, g := range apiGroups.Groups {
		// The group is the same for all known versions: v1, v1beta1, v1beta2
		// so we can use the variable from any package.
		if g.Name == gv.Group {
			return hasMatchingGroupVersions(g.Versions, gv.Version), nil
		}
	}
	return false, nil
}

// ListVPAResources returns the list of VerticalPodAutoscaler
// resources that match the listing options parameters.
func (c *client) ListVPAResources(_ context.Context, opts ListOptions) ([]*vpav1.VerticalPodAutoscaler, error) {
	b := resource.NewBuilder(c.flags)
	r := b.Unstructured().
		NamespaceParam(opts.Namespace).
		AllNamespaces(opts.AllNamespaces).
		LabelSelectorParam(opts.LabelSelector).
		FieldSelectorParam(opts.FieldSelector).
		RequestChunksOf(opts.Limit).
		ResourceTypeOrNameArgs(true, append([]string{vpaKind}, opts.ResourceNames...)...).
		SingleResourceType().
		RequireObject(true).
		ContinueOnError().
		Flatten().
		Latest().
		Do()

	if err := r.Err(); err != nil {
		return nil, err
	}
	infos, err := r.Infos()
	if err != nil || infos == nil {
		return nil, err
	}
	conv := runtime.DefaultUnstructuredConverter
	vpas := make([]*vpav1.VerticalPodAutoscaler, len(infos))

	for i, v := range infos {
		// v.Object cannot be nil when the builder was
		// created with the RequireObject(true) method.
		u, ok := v.Object.(*unstructuredv1.Unstructured)
		if !ok {
			return nil, fmt.Errorf("unexpected result type: %T", v)
		}
		vpas[i] = &vpav1.VerticalPodAutoscaler{}
		if err := conv.FromUnstructured(u.Object, vpas[i]); err != nil {
			return nil, err
		}
	}
	return vpas, nil
}

// GetVPATarget fetches the controller targeted by the given VPA reference
// and return a generic unstructured object.
func (c *client) GetVPATarget(ctx context.Context, ref *autoscalingv1.CrossVersionObjectReference, namespace string) (*unstructuredv1.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q into GroupVersion: %w", ref.APIVersion, err)
	}
	gk := schema.GroupKind{
		Group: gv.Group,
		Kind:  ref.Kind,
	}
	m, err := c.mapper.RESTMapping(gk, gv.Version)
	if err != nil {
		return nil, fmt.Errorf("couldn't find mapping for %s: %w", gk.WithVersion(gv.Version), err)
	}
	var ri dynamic.ResourceInterface

	nri := c.dynamicClient.Resource(m.Resource)

	// The target reference of a VPA spec has no namespace field.
	// We assume that the reference is for a resource in the same
	// namespace as the VPA if the scope resource is namespace.
	if m.Scope.Name() == meta.RESTScopeNameNamespace {
		ri = nri.Namespace(namespace)
	} else {
		ri = nri
	}
	obj, err := ri.Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		switch {
		case apierrors.IsForbidden(err):
			return nil, fmt.Errorf("no access to get resource %s in namespace %s", m.Resource.String(), namespace)
		case apierrors.IsNotFound(err):
			return nil, fmt.Errorf("resource not found: %s/%s", namespace, ref.Name)
		default:
			return nil, fmt.Errorf("couldn't get resource %s in namespace %s: %w", m.Resource.String(), namespace, err)
		}
	}
	return obj, nil
}

// ListDependentPods returns the list of pods that depends
// on the controller represented by its metadata.
func (c *client) ListDependentPods(ctx context.Context, targetMeta metav1.ObjectMeta) ([]*corev1.Pod, error) {
	i := c.coreClient.Pods(targetMeta.Namespace)

	p := pager.New(func(ctx context.Context, o metav1.ListOptions) (runtime.Object, error) {
		return i.List(ctx, o)
	})
	obj, _, err := p.List(ctx, metav1.ListOptions{
		Limit: 250,
	})
	if err != nil {
		return nil, err
	}
	list := obj.(*corev1.PodList)
	pods := make([]*corev1.Pod, 0)

	for i, pod := range list.Items {
		for _, ref := range pod.GetOwnerReferences() {
			// TODO(will): This is rather weak.
			// Instead, we should go through the chain of owner references
			// to find the top-most controller that match the target ref
			// of the VerticalPodAutoscaler resource.
			// i.e. Pod -> ReplicaSet -> Deployment
			if referenceMatchController(ref, targetMeta) {
				pods = append(pods, &list.Items[i])
			}
		}
	}
	return pods, nil
}

// referenceMatchController returns whether the given
// OwnerReference matches a target controller.
func referenceMatchController(ref metav1.OwnerReference, targetMeta metav1.ObjectMeta) bool {
	return strings.HasPrefix(ref.Name, targetMeta.Name)
}

// hasMatchingGroupVersions returns whether the group versions lists match.
func hasMatchingGroupVersions(groupVersions []metav1.GroupVersionForDiscovery, wantVersions ...string) bool {
	b := false
L:
	for _, v := range wantVersions {
		for _, gv := range groupVersions {
			if gv.Version == v {
				b = true
				continue L
			}
		}
		// If we reach this point, we could not match the
		// wanted version with one of the group versions.
		b = false
	}
	return b
}
