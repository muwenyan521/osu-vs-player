package overlays

import (
	color2 "github.com/wieku/danser-go/framework/math/color"
	"testing"
)

func TestDistinctPlayerPalette68(t *testing.T) {
	a, b := distinctPlayerPalette(68), distinctPlayerPalette(68)
	if len(a) != 68 {
		t.Fatalf("palette has %d colors", len(a))
	}
	closest := 1.0
	for i, c := range a {
		if c != b[i] {
			t.Fatal("palette is not deterministic")
		}
		for j := 0; j < i; j++ {
			d := playerColorDistance(c, a[j])
			if d < closest {
				closest = d
			}
			if d < 0.000001 {
				t.Fatalf("players %d and %d have near-identical colors", i, j)
			}
		}
	}
	t.Logf("minimum OKLab distance: %.5f", closest)
}

func TestPlayerPaletteAvoidsDarkColors(t *testing.T) {
	for i, c := range distinctPlayerPalette(100) {
		if got := playerColorLuminance(c); got < 0.0625-1e-6 {
			t.Fatalf("color %d luminance %f below visibility floor", i, got)
		}
	}
}

func TestDarkColorLiftLeavesBrighterColorsUnchanged(t *testing.T) {
	for _, c := range []color2.Color{color2.NewRGBA(1, 0, 0, 0.5), color2.NewRGBA(0, 1, 0, 1), color2.NewRGBA(0, 1, 1, 1)} {
		if got := liftDarkPlayerColor(c); got != c {
			t.Fatalf("changed brighter color: %v -> %v", c, got)
		}
	}
	dark := color2.NewRGBA(0, 0, 0.55, 0.5)
	got := liftDarkPlayerColor(dark)
	if playerColorLuminance(got) < 0.0625-1e-6 || got.A != dark.A || got.B <= got.R {
		t.Fatalf("bad dark color lift: %v", got)
	}
}
