package overlays

import "testing"

func TestDifficultyRole(t *testing.T) {
	for _, tc := range []struct {
		current float64
		all     []float64
		want    string
	}{
		{10, []float64{8, 10, 9}, "主难"}, {9, []float64{8, 10, 9}, "次难"}, {8, []float64{8, 10, 9}, ""}, {10, []float64{10, 10, 9}, "主难"},
	} {
		if got := difficultyRole(tc.current, tc.all); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}

func TestIntroCardHoldsThreeSeconds(t *testing.T) {
	for _, remaining := range []float64{450, 1500, 3450} {
		if introCardAlpha(remaining) != 1 {
			t.Fatal("card must be fully visible for 3000ms")
		}
	}
	for _, remaining := range []float64{0, 3900, 4000} {
		if introCardAlpha(remaining) != 0 {
			t.Fatal("card must be hidden outside its window")
		}
	}
}
