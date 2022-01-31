package vpa

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

// RecommendationType represents a type of container resources
// recommendation made by a VPA recommender.
type RecommendationType string

// VPA recommendation types.
const (
	RecommendationTarget         RecommendationType = "target"
	RecommendationLowerBound     RecommendationType = "lower-bound"
	RecommendationUpperBound     RecommendationType = "upper-bound"
	RecommendationUncappedTarget RecommendationType = "uncapped-target"
)

// String implements the pflag.Value interface.
func (rt RecommendationType) String() string { return string(rt) }

// Type implements the pflag.Value interface.
func (rt *RecommendationType) Type() string { return "string" }

// Set implements the pflag.Value interface.
func (rt *RecommendationType) Set(s string) error {
	switch RecommendationType(s) {
	case RecommendationTarget, RecommendationLowerBound, RecommendationUpperBound, RecommendationUncappedTarget:
		*rt = RecommendationType(s)
		return nil
	default:
		return fmt.Errorf("must be one of: %s, %s, %s or %s",
			RecommendationTarget,
			RecommendationLowerBound,
			RecommendationUpperBound,
			RecommendationUncappedTarget,
		)
	}
}

// TotalRecommendations returns the total resource recommendations
// of the given VPA as the sum of all resource quantities recommended
// for each container.
func TotalRecommendations(vpa *vpav1.VerticalPodAutoscaler, rt RecommendationType) ResourceQuantities {
	if vpa == nil || vpa.Status.Recommendation == nil {
		return ResourceQuantities{}
	}
	var cpu, mem resource.Quantity
	for _, cr := range vpa.Status.Recommendation.ContainerRecommendations {
		rec := recommendationsByType(cr, rt)

		if c := rec.Cpu(); c != nil {
			cpu.Add(*c)
		}
		if m := rec.Memory(); m != nil {
			mem.Add(*m)
		}
	}
	return ResourceQuantities{
		CPU:    &cpu,
		Memory: &mem,
	}
}

func recommendationsByType(rec vpav1.RecommendedContainerResources, rt RecommendationType) v1.ResourceList {
	switch rt {
	case RecommendationTarget:
		return rec.Target
	case RecommendationLowerBound:
		return rec.LowerBound
	case RecommendationUpperBound:
		return rec.UpperBound
	case RecommendationUncappedTarget:
		return rec.UncappedTarget
	}
	return rec.Target
}
