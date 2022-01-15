package vpa

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

func TestDiffQuantitiesAsPercent(t *testing.T) {
	testCases := []struct {
		X *resource.Quantity
		Y *resource.Quantity
		P *float64
	}{
		{
			X: resource.NewQuantity(0, resource.DecimalSI),
			Y: resource.NewQuantity(25, resource.DecimalSI),
			P: nil,
		},
		{
			X: resource.NewQuantity(25, resource.DecimalSI),
			Y: resource.NewQuantity(0, resource.DecimalSI),
			P: nil,
		},
		{
			X: resource.NewQuantity(100, resource.DecimalSI),
			Y: resource.NewQuantity(25, resource.DecimalSI),
			P: pointer.Float64(75.00),
		},
		{
			X: resource.NewQuantity(444, resource.BinarySI),
			Y: resource.NewQuantity(1000, resource.BinarySI),
			P: pointer.Float64(-125.23),
		},
		{
			X: resource.NewQuantity(1000, resource.BinarySI),
			Y: resource.NewQuantity(444, resource.BinarySI),
			P: pointer.Float64(55.60),
		},
		{
			X: resource.NewScaledQuantity(8, resource.Peta),
			Y: resource.NewScaledQuantity(24, resource.Peta),
			P: pointer.Float64(-200.00),
		},
		{
			X: resource.NewScaledQuantity(128, resource.Exa),
			Y: resource.NewScaledQuantity(512, resource.Exa),
			P: pointer.Float64(-300.00),
		},
		{
			X: resource.NewScaledQuantity(666, resource.Giga),
			Y: resource.NewScaledQuantity(32, resource.Exa),
			P: pointer.Float64(-4804804704.80),
		},
		{
			X: resource.NewScaledQuantity(32, resource.Exa),
			Y: resource.NewScaledQuantity(666, resource.Giga),
			P: pointer.Float64(99.99),
		},
	}
	for _, tc := range testCases {
		t.Logf("X: %s, Y: %s", tc.X, tc.Y)

		p := DiffQuantitiesAsPercent(tc.X, tc.Y)
		if p == nil {
			if tc.P != nil {
				t.Errorf("got nil result, want %.2f", *tc.P)
			}
		} else {
			if *tc.P != *p {
				t.Errorf("got %f, want %f", *p, *tc.P)
			}
		}
	}
}
