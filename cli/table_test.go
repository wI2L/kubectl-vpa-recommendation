package cli

import (
	"bytes"
	"fmt"
	"testing"
	"text/tabwriter"

	"k8s.io/utils/pointer"
)

func TestSortTable(t *testing.T) {
	table := table{
		{
			Name:             "zeus",
			Namespace:        "athens",
			CPUDifference:    pointer.Float64(3.14),
			MemoryDifference: pointer.Float64(12.34),
		},
		{
			Name:             "hera",
			Namespace:        "sparta",
			CPUDifference:    pointer.Float64(1.43),
			MemoryDifference: pointer.Float64(-6.72),
		},
		{
			Name:             "poseidon",
			Namespace:        "corinth",
			CPUDifference:    pointer.Float64(-2.50),
			MemoryDifference: pointer.Float64(8.9),
		},
		{
			Name:             "demeter",
			Namespace:        "pergamon",
			CPUDifference:    nil,
			MemoryDifference: pointer.Float64(0.45),
		},
		{
			Name:             "athena",
			Namespace:        "athens",
			CPUDifference:    nil,
			MemoryDifference: nil,
		},
		{
			Name:             "apollo",
			Namespace:        "olympia",
			CPUDifference:    pointer.Float64(10.21),
			MemoryDifference: pointer.Float64(13.37),
		},
		{
			Name:             "artemis",
			Namespace:        "thebes",
			CPUDifference:    pointer.Float64(-0.01),
			MemoryDifference: pointer.Float64(7.43),
		},
		{
			Name:             "hermes",
			Namespace:        "sparta",
			CPUDifference:    pointer.Float64(-100.43),
			MemoryDifference: nil,
		},
		{
			Name:             "dionysus",
			Namespace:        "corinth",
			CPUDifference:    pointer.Float64(-250.00),
			MemoryDifference: pointer.Float64(666.66),
		},
	}
	table.SortBy(orderAsc, "namespace", "name")

	orderedRows := []struct{ name, namespace string }{
		{
			"athens",
			"athena",
		},
		{
			"athens",
			"zeus",
		},
		{
			"corinth",
			"dionysus",
		},
		{
			"corinth",
			"poseidon",
		},
		{
			"olympia",
			"apollo",
		},
		{
			"pergamon",
			"demeter",
		},
		{
			"sparta",
			"hera",
		},
		{
			"sparta",
			"hermes",
		},
		{
			"thebes",
			"artemis",
		},
	}
	for i, v := range orderedRows {
		if v.name != table[i].Namespace {
			t.Errorf("expected name %s, got %s", v.name, table[i].Name)
		}
		if v.name != table[i].Namespace {
			t.Errorf("expected namespace %s, got %s", v.name, table[i].Name)
		}
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 1, 0, 2, ' ', 0)

	for _, row := range table {
		_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", row.Namespace, row.Name, fts(row.CPUDifference), fts(row.MemoryDifference))
		if err != nil {
			t.Error(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Error(err)
	}
	t.Logf("\n%s", buf.String())
}

func fts(f *float64) string {
	if f == nil {
		return "-"
	}
	return fmt.Sprintf("%+.2f", *f)
}
