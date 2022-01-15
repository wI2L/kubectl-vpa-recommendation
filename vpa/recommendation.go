package vpa

import (
	"k8s.io/apimachinery/pkg/api/resource"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

// GetTotalRecommendations returns the total resource recommendations
// of the given VPA as the sum of all resource quantities recommended
// for each container.
func GetTotalRecommendations(vpa *vpav1.VerticalPodAutoscaler) *ResourceQuantities {
	if vpa == nil || vpa.Status.Recommendation == nil {
		return nil
	}
	var cpu, mem resource.Quantity
	for _, cr := range vpa.Status.Recommendation.ContainerRecommendations {
		if c := cr.Target.Cpu(); c != nil {
			cpu.Add(*c)
		}
		if m := cr.Target.Memory(); m != nil {
			mem.Add(*m)
		}
	}
	return &ResourceQuantities{
		CPU:    &cpu,
		Memory: &mem,
	}
}
