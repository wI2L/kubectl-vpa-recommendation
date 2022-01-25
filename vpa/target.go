package vpa

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/wI2L/kubectl-vpa-recommendation/client"
)

// wellKnownControllerKind represents the Kind of common controllers.
type wellKnownControllerKind string

const (
	cj     wellKnownControllerKind = "CronJob"
	ds     wellKnownControllerKind = "DaemonSet"
	deploy wellKnownControllerKind = "Deployment"
	node   wellKnownControllerKind = "Node"
	job    wellKnownControllerKind = "Job"
	rs     wellKnownControllerKind = "ReplicaSet"
	rc     wellKnownControllerKind = "ReplicationController"
	sts    wellKnownControllerKind = "StatefulSet"
)

// TargetController abstract a scalable controller
// resource targeted by a VerticalPodAutoscaler.
type TargetController struct {
	Name             string
	Namespace        string
	GroupVersionKind schema.GroupVersionKind
	controllerKind   wellKnownControllerKind
	controllerObj    *unstructuredv1.Unstructured
	podSpec          *corev1.PodSpec
}

// NewTargetController resolves the target of a VPA resource.
func NewTargetController(c client.Interface, ref *autoscalingv1.CrossVersionObjectReference, namespace string) (*TargetController, error) {
	ctx := context.Background()

	obj, err := c.GetVPATarget(ctx, ref, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch VPa target: %w", err)
	}
	kind := obj.GetKind()

	switch wellKnownControllerKind(kind) {
	case cj, ds, deploy, job, rs, rc, sts:
	case node:
		// Some pods specify nodes as their owners,
		// but they aren't valid controllers that
		// the VPA supports, so we just skip them.
		return nil, fmt.Errorf("node is not a valid target")
	default:
		return nil, fmt.Errorf("unsupported target kind: %s", kind)
	}
	tc := &TargetController{
		Name:             obj.GetName(),
		Namespace:        obj.GetNamespace(),
		GroupVersionKind: obj.GetObjectKind().GroupVersionKind(),
		controllerKind:   wellKnownControllerKind(kind),
		controllerObj:    obj,
	}
	tc.podSpec, err = resolvePodSpec(obj)
	if err != nil {
		return nil, err
	}
	labelSelector, err := resolveLabelSelector(obj)
	if err != nil {
		return nil, err
	}
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, err
	}
	// The PodSpec template defined by a controller might not represent
	// the final spec of the pods. For example, a LimitRanger controller
	// could change the spec of pods to set default resource requests and
	// limits. To ensure that we have a reliable comparison source, we have
	// no choice but to list the pods and find those who are dependents of
	// the controller to get the most up-to-date spec.
	m, _, err := unstructuredv1.NestedMap(obj.Object, "metadata")
	if err != nil {
		return nil, err
	}
	meta := metav1.ObjectMeta{}
	conv := runtime.DefaultUnstructuredConverter

	if err := conv.FromUnstructured(m, &meta); err != nil {
		return nil, err
	}
	pods, err := c.ListDependentPods(context.Background(), meta, selector.String())
	if err != nil {
		return nil, err
	}
	if len(pods) != 0 {
		p := pods[0]
		if !reflect.DeepEqual(p.Spec.Containers, tc.podSpec.Containers) {
			tc.podSpec = &p.Spec
		}
	}
	return tc, nil
}

// GetContainerRequests returns the resource requests of a container.
func (tc *TargetController) GetContainerRequests(name string) ResourceQuantities {
	for _, c := range tc.podSpec.Containers {
		if c.Name == name {
			return ResourceQuantities{
				CPU:    c.Resources.Requests.Cpu(),
				Memory: c.Resources.Requests.Memory(),
			}
		}
	}
	return ResourceQuantities{}
}

// GetRequests returns the resource requests defined by the
// pod spec of the controller, which is the sum of all resource
// quantities for each container declared by the spec.
func (tc *TargetController) GetRequests() ResourceQuantities {
	var cpu, mem resource.Quantity

	for _, ctr := range tc.podSpec.Containers {
		requests := ctr.Resources.Requests
		if c := requests.Cpu(); c != nil {
			cpu.Add(*c)
		}
		if m := requests.Memory(); m != nil {
			mem.Add(*m)
		}
	}
	return ResourceQuantities{CPU: &cpu, Memory: &mem}
}

// resolvePodSpec returns the corev1.PodSpec field of a controller.
func resolvePodSpec(obj *unstructuredv1.Unstructured) (*corev1.PodSpec, error) {
	fields := []string{
		"spec",
		"template", // PodTemplateSpec
		"spec",     // PodSpec
	}
	var err error
	fields, err = genericControllerSpecPath(obj.GetKind(), fields)
	if err != nil {
		return nil, err
	}
	spec := &corev1.PodSpec{}
	if err := decodeNestedFieldInto(obj, fields, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

// resolveLabelSelector returns the metav1.LabelSelector field of a controller spec.
func resolveLabelSelector(obj *unstructuredv1.Unstructured) (*metav1.LabelSelector, error) {
	fields := []string{
		"spec",
		"selector",
	}
	var err error
	fields, err = genericControllerSpecPath(obj.GetKind(), fields)
	if err != nil {
		return nil, err
	}
	selector := &metav1.LabelSelector{}
	if err := decodeNestedFieldInto(obj, fields, selector); err != nil {
		return nil, err
	}
	return selector, nil
}

func genericControllerSpecPath(kind string, fields []string) ([]string, error) {
	switch wellKnownControllerKind(kind) {
	case ds, deploy, job, rs, rc, sts:
		// Same default fields.
	case cj:
		prefix := []string{
			"spec",        // CronJobSpec
			"jobTemplate", // JobTemplateSpec
		}
		return append(prefix, fields...), nil
	default:
		return nil, fmt.Errorf("unknown kind: %s", kind)
	}
	return fields, nil
}

func decodeNestedFieldInto(obj *unstructuredv1.Unstructured, fields []string, into interface{}) error {
	nmap, ok, err := unstructuredv1.NestedMap(obj.Object, fields...)
	if err != nil {
		return fmt.Errorf("nested field has invalid type")
	}
	if !ok {
		return fmt.Errorf("nested field with path %s not found", strings.Join(fields, "."))
	}
	conv := runtime.DefaultUnstructuredConverter

	return conv.FromUnstructured(nmap, into)
}
