package postgres_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/innermond/dots/postgres"
)

var testcases = []struct {
	// existent entries quantities for an entry type
	existent map[int]map[int]float64
	//desired qunatity for an entry type to be covered by existent quantities
	desired map[int]float64
	// expected distribution
	expected map[int]float64
}{
	{
		map[int]map[int]float64{
			1: {10: 10, 11: 10, 12: 10},
		},
		map[int]float64{1: 15},
		map[int]float64{10: 10, 11: 5},
	},
	{
		map[int]map[int]float64{
			1: {10: 10, 11: 10, 12: 10},
			2: {13: 10, 14: 10, 12: 10},
		},
		map[int]float64{1: 15, 2: 10},
		map[int]float64{10: 10, 11: 5, 13: 10},
	},
	{
		map[int]map[int]float64{
			1: {1: 1, 2: 1},
			2: {3: 1, 4: 1},
		},
		map[int]float64{1: 2, 2: 2},
		map[int]float64{1: 1, 2: 1, 3: 1, 4: 1},
	},
}

func TestDistributeFrom(t *testing.T) {
	for i, tc := range testcases {
		t.Run(fmt.Sprintf("%d:", i), func(t *testing.T) {
			distribute, err := postgres.DistributeFrom(tc.existent, tc.desired)
			if err != nil {
				t.Fatalf("unexpected: %v\n", err)
			}
			if !reflect.DeepEqual(distribute, tc.expected) {
				t.Logf("expected: %v got %v", tc.expected, distribute)
			}
		})
	}
}
