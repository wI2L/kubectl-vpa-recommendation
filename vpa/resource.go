package vpa

import (
	"math"
	"strconv"

	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"
)

const precision = 2

// ResourceQuantities is a pair of resource quantities
// that can represent the recommendations of a VerticalPodAutoscaler
// of the requests of a pod's container.
type ResourceQuantities struct {
	CPU    *resource.Quantity
	Memory *resource.Quantity
}

// DiffQuantitiesAsPercent return the difference between
// two quantities expressed as a percentage of each other,
// where x is the request and y the recommendation.
func DiffQuantitiesAsPercent(x, y *resource.Quantity) *float64 {
	if x == nil || y == nil || x.IsZero() || y.IsZero() {
		return nil
	}
	ai, oka := x.AsInt64()
	bi, okb := y.AsInt64()

	if oka && okb {
		f := (float64(ai) - float64(bi)) / float64(ai) * 100
		exp := powFloat64(10, precision)
		// Round down to 'precision' decimal places.
		f = math.Floor(f*exp) / exp
		return &f
	}
	ad := x.AsDec()
	bd := y.AsDec()

	p := &inf.Dec{}
	p.Sub(ad, bd)
	p.QuoRound(p, ad, precision+precision, inf.RoundDown)
	p.Mul(p, inf.NewDec(100, 0))
	p.Round(p, precision, inf.RoundDown)

	f, err := strconv.ParseFloat(p.String(), 64)
	if err != nil {
		return nil
	}
	return &f
}

func powFloat64(x, y int) float64 {
	return math.Pow(float64(x), float64(y))
}
