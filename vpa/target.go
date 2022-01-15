package vpa

import (
	"context"
	"fmt"
	"strings"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/wI2L/kubectl-commitment/client"
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
func NewTargetController(client client.Interface, ref *autoscalingv1.CrossVersionObjectReference, namespace string) (*TargetController, error) {
	ctx := context.Background()

	obj, err := client.GetVPATarget(ctx, ref, namespace)
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
	return tc, nil
}

// GetRequests returns the resource requests defined by the
// pod spec of the controller, which is the sum of all resource
// quantities for each container declared by the spec.
func (tc *TargetController) GetRequests() *ResourceQuantities {
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
	return &ResourceQuantities{
		CPU:    &cpu,
		Memory: &mem,
	}
}

// GetContainerRequests returns the resource requests of a container.
func (tc *TargetController) GetContainerRequests(name string) *ResourceQuantities {
	for _, c := range tc.podSpec.Containers {
		if c.Name == name {
			return &ResourceQuantities{
				CPU:    c.Resources.Requests.Cpu(),
				Memory: c.Resources.Requests.Memory(),
			}
		}
	}
	return nil
}

// resolvePodSpec returns the corev1.PodSpec field of a
// controller. The method cache the result during its first
// call and return the same value for subsequent calls.
func resolvePodSpec(obj *unstructuredv1.Unstructured) (*corev1.PodSpec, error) {
	kind := obj.GetKind()

	fields := []string{
		"spec",
		"template", // PodTemplateSpec
		"spec",     // PodSpec
	}
	switch wellKnownControllerKind(kind) {
	case ds, deploy, job, rs, rc, sts:
		// Same default fields.
	case cj:
		prefix := []string{
			"spec",        // CronJobSpec
			"jobTemplate", // JobTemplateSpec
		}
		fields = append(prefix, fields...)
	default:
		return nil, fmt.Errorf("unknown kind: %s", kind)
	}
	nmap, ok, err := unstructuredv1.NestedMap(obj.Object, fields...)
	if err != nil {
		return nil, fmt.Errorf("nested field has invalid type")
	}
	if !ok {
		return nil, fmt.Errorf("nested field with path %s not found", strings.Join(fields, "."))
	}
	conv := runtime.DefaultUnstructuredConverter
	spec := &corev1.PodSpec{}

	err = conv.FromUnstructured(nmap, spec)
	if err != nil {
		return nil, err
	}
	return spec, nil
}
