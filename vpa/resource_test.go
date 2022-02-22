package vpa

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

func TestDiffQuantitiesAsPercent(t *testing.T) {
	testCases := []struct {
		Req *resource.Quantity
		Rec *resource.Quantity
		P   *float64
	}{
		{
			Req: resource.NewQuantity(4, resource.DecimalSI),
			Rec: resource.NewQuantity(1, resource.DecimalSI),
			P:   pointer.Float64(300.00),
		},
		{
			Req: resource.NewQuantity(100, resource.DecimalSI),
			Rec: resource.NewQuantity(25, resource.DecimalSI),
			P:   pointer.Float64(300.00),
		},
		{
			Req: resource.NewQuantity(444, resource.BinarySI),
			Rec: resource.NewQuantity(1000, resource.BinarySI),
			P:   pointer.Float64(-55.60),
		},
		{
			Req: resource.NewQuantity(1000, resource.BinarySI),
			Rec: resource.NewQuantity(444, resource.BinarySI),
			P:   pointer.Float64(125.23),
		},
		{
			Req: resource.NewScaledQuantity(8, resource.Peta),
			Rec: resource.NewScaledQuantity(24, resource.Peta),
			P:   pointer.Float64(-66.67),
		},
		{
			Req: resource.NewScaledQuantity(128, resource.Exa),
			Rec: resource.NewScaledQuantity(512, resource.Exa),
			P:   pointer.Float64(-75.00),
		},
		{
			Req: resource.NewScaledQuantity(666, resource.Giga),
			Rec: resource.NewScaledQuantity(32, resource.Exa),
			P:   pointer.Float64(-100.00),
		},
		{
			Req: resource.NewScaledQuantity(32, resource.Exa),
			Rec: resource.NewScaledQuantity(666, resource.Giga),
			P:   pointer.Float64(4804804704.80),
		},
		{
			Req: resource.NewMilliQuantity(100, resource.DecimalSI),
			Rec: resource.NewMilliQuantity(25, resource.DecimalSI),
			P:   pointer.Float64(300.00),
		},
	}
	for _, tc := range testCases {
		t.Logf("Req: %s, Rec: %s", tc.Req, tc.Rec)

		p := DiffQuantitiesAsPercent(tc.Req, tc.Rec)
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
