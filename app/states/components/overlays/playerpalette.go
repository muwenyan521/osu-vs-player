package overlays

import (
	color2 "github.com/wieku/danser-go/framework/math/color"
	"math"
)

// Pick the next colour farthest from the colours already used, in OKLab.
// Varying lightness and saturation avoids repeating a short hue-only cycle.
func distinctPlayerPalette(count int) []color2.Color {
	candidates := make([]color2.Color, 0, 720)
	for h := 0; h < 360; h += 3 {
		for _, sv := range [][2]float32{{0.9, 1}, {0.6, 1}, {0.9, 0.72}, {0.55, 0.78}, {0.3, 1}, {1, 0.55}} {
			candidates = append(candidates, color2.NewHSV(float32(h), sv[0], sv[1]))
		}
	}
	palette := make([]color2.Color, 0, count)
	distances := make([]float64, len(candidates))
	for i := range distances {
		distances[i] = math.Inf(1)
	}
	selected := 0
	for len(palette) < count && len(palette) < len(candidates) {
		chosen := candidates[selected]
		palette = append(palette, chosen)
		best := -1.0
		next := 0
		for i, c := range candidates {
			distances[i] = math.Min(distances[i], playerColorDistance(c, chosen))
			if distances[i] > best {
				best = distances[i]
				next = i
			}
		}
		selected = next
	}
	for i, c := range palette {
		palette[i] = liftDarkPlayerColor(c)
	}
	return palette
}

func playerColorDistance(a, b color2.Color) float64 {
	lab := func(c color2.Color) [3]float64 {
		linear := func(v float32) float64 {
			x := float64(v)
			if x <= 0.04045 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		r, g, b := linear(c.R), linear(c.G), linear(c.B)
		l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
		m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
		s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
		return [3]float64{0.2104542553*l + 0.793617785*m - 0.0040720468*s, 1.9779984951*l - 2.428592205*m + 0.4505937099*s, 0.0259040371*l + 0.7827717662*m - 0.808675766*s}
	}
	x, y := lab(a), lab(b)
	d := 0.0
	for i := range x {
		d += (x[i] - y[i]) * (x[i] - y[i])
	}
	return d
}

// Linear sRGB relative luminance.
func playerColorLuminance(c color2.Color) float64 {
	linear := func(v float32) float64 {
		x := float64(v)
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

// Preserve the original palette selection and all but its darkest colors.
func liftDarkPlayerColor(c color2.Color) color2.Color {
	return liftPlayerColorToLuminance(c, 0.0625)
}

func liftPlayerColorToLuminance(c color2.Color, floor float64) color2.Color {
	if playerColorLuminance(c) >= floor {
		return c
	}
	original := c
	low, high := 0.0, 1.0
	mix := func(t float64) color2.Color {
		c := original
		c.R += (1 - c.R) * float32(t)
		c.G += (1 - c.G) * float32(t)
		c.B += (1 - c.B) * float32(t)
		return c
	}
	for i := 0; i < 24; i++ {
		middle := (low + high) / 2
		if playerColorLuminance(mix(middle)) < floor {
			low = middle
		} else {
			high = middle
		}
	}
	return mix(high)
}
