package vpa

import (
	"math"

	"k8s.io/apimachinery/pkg/api/resource"
)

// ResourceQuantities is a pair of resource quantities
// that can represent the recommendations of a VerticalPodAutoscaler
// of the requests of a pod's container.
type ResourceQuantities struct {
	CPU    *resource.Quantity
	Memory *resource.Quantity
}

// DiffQuantitiesAsPercent return the difference between two
// quantities. The return value is expressed as the increase/decrease
// of the request in terms of the recommendation.
func DiffQuantitiesAsPercent(req, rec *resource.Quantity) *float64 {
	if req == nil || rec == nil || req.IsZero() || rec.IsZero() {
		return nil
	}
	xf := req.AsApproximateFloat64()
	yf := rec.AsApproximateFloat64()

	p := (xf - yf) / yf * 100.0
	p = math.Round(p*100) / 100 // round nearest

	return &p
}
