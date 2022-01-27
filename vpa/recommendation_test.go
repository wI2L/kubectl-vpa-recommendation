package vpa

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/resource"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

func TestParseRecommendationTypeFlag(t *testing.T) {
	testcases := []struct {
		Argv    string
		Value   RecommendationType
		Success bool
	}{
		{
			"--recommendation-type target",
			RecommendationTarget,
			true,
		},
		{
			"--recommendation-type=lower-bound",
			RecommendationLowerBound,
			true,
		},
		{
			"--recommendation-type=lower-bound --recommendation-type upper-bound",
			RecommendationUpperBound,
			true,
		},
		{
			"--recommendation-type target --recommendation-type=uncapped-target",
			RecommendationUncappedTarget,
			true,
		},
		{
			"--recommendation-type foo",
			"",
			false,
		},
		{
			"--recommendation-type=bar",
			"",
			false,
		},
		{
			"--recommendation-type invalid --recommendation-type target",
			"",
			false,
		},
		{
			"--recommendation-type=uncapped-target --recommendation-type=invalid",
			"",
			false,
		},
	}
	for _, tc := range testcases {
		var rt RecommendationType

		fs := pflag.NewFlagSet("", pflag.ContinueOnError)
		fs.Var(&rt, "recommendation-type", "")

		err := fs.Parse(strings.Split(tc.Argv, " "))
		if err != nil && tc.Success {
			t.Fatal(err)
		}
		if err == nil {
			if !tc.Success {
				t.Fatalf("expected a parse error but it succeeded")
			} else if tc.Value != rt {
				t.Errorf("got %q, want %q", rt, tc.Value)
			}
		}
	}
}

func TestTotalRecommendations(t *testing.T) {
	f, err := os.Open("testdata/vpa.json")
	if err != nil {
		t.Fatal(err)
	}
	vpa := &vpav1.VerticalPodAutoscaler{}
	dec := json.NewDecoder(f)

	if err = dec.Decode(vpa); err != nil {
		t.Fatal(err)
	}
	t.Run("valid", func(t *testing.T) {
		for _, tc := range []struct {
			Type   RecommendationType
			CPU    *resource.Quantity
			Memory *resource.Quantity
		}{
			{
				RecommendationLowerBound,
				resource.NewScaledQuantity(65, resource.Milli),
				resource.NewScaledQuantity(132, resource.Mega),
			},
			{
				RecommendationTarget,
				resource.NewScaledQuantity(350, resource.Milli),
				resource.NewScaledQuantity(184, resource.Mega),
			},
			{
				RecommendationUncappedTarget,
				resource.NewScaledQuantity(650, resource.Milli),
				resource.NewScaledQuantity(288, resource.Mega),
			},
			{
				RecommendationUpperBound,
				resource.NewScaledQuantity(966, resource.Milli),
				resource.NewScaledQuantity(436, resource.Mega),
			},
		} {
			recos := TotalRecommendations(vpa, tc.Type)

			if recos.CPU == nil {
				t.Fatal("expected non-nil cpu quantity")
			}
			if recos.Memory == nil {
				t.Fatal("expected non-nil mem quantity")
			}
			t.Logf("\n%s\n\tcpu: %v\n\tmem: %v", tc.Type, recos.CPU, recos.Memory)

			if !recos.CPU.Equal(*tc.CPU) {
				t.Errorf("got cpu=%q, want %q", recos.CPU, tc.CPU)
			}
			if !recos.Memory.Equal(*tc.Memory) {
				t.Errorf("got mem=%q, want %q", recos.Memory, tc.Memory)
			}
		}
	})
	t.Run("invalid", func(t *testing.T) {
		vpa.Status.Recommendation = nil // test with no recommendation

		for _, tc := range []struct {
			Recommendations ResourceQuantities
		}{
			{TotalRecommendations(nil, RecommendationTarget)},
			{TotalRecommendations(nil, RecommendationLowerBound)},
		} {
			if tc.Recommendations.CPU != nil || tc.Recommendations.Memory != nil {
				t.Fatal("expected cpu/mem recommendation to be nil")
			}
		}
	})
}
