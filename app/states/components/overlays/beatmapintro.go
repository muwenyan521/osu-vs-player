package overlays

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/wieku/danser-go/app/beatmap"
	"github.com/wieku/danser-go/app/graphics"
	"github.com/wieku/danser-go/app/rulesets/osu/performance/pp260706"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/graphics/batch"
	"github.com/wieku/danser-go/framework/math/vector"
)

type beatmapIntro struct {
	title, artist, difficulty, creator, role, details, timing string
	stars                                                     float64
	end                                                       float64
}

func difficultyRole(stars float64, others []float64) string {
	higher := 0
	for _, other := range others {
		if other > stars+1e-6 {
			higher++
		}
	}
	if higher == 0 {
		return "主难"
	}
	if higher == 1 {
		return "次难"
	}
	return ""
}

func loadBeatmapIntro(current *beatmap.BeatMap) *beatmapIntro {
	calc := pp260706.NewDifficultyCalculator()
	files, err := filepath.Glob(filepath.Join(settings.General.GetSongsDir(), current.Dir, "*.osu"))
	if err != nil {
		panic(err)
	}
	stars := []float64{}
	currentStars := -1.0
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		m := beatmap.ParseBeatMapFile(f)
		f.Close()
		if m == nil {
			panic(fmt.Sprintf("cannot parse difficulty for opening card: %s", path))
		}
		if m.Mode != current.Mode || (current.SetID > 0 && m.SetID != current.SetID) {
			continue
		}
		beatmap.ParseTimingPointsAndPauses(m)
		beatmap.ParseObjects(m, true, false)
		value := calc.CalculateSingle(m, m.Diff).Total
		log.Printf("Intro difficulty: %s = %.6f stars (NM)", m.Difficulty, value)
		stars = append(stars, value)
		if m.File == current.File {
			currentStars = value
		}
	}
	if currentStars < 0 {
		panic("current difficulty missing from beatmap intro comparison")
	}
	d := current.Diff
	bpm := fmt.Sprintf("BPM %.0f", current.MaxBPM)
	if math.Abs(current.MaxBPM-current.MinBPM) > 0.1 {
		bpm = fmt.Sprintf("BPM %.0f–%.0f", current.MinBPM, current.MaxBPM)
	}
	duration := int(current.HitObjects[len(current.HitObjects)-1].GetEndTime() / 1000)
	return &beatmapIntro{title: current.Name, artist: current.Artist, difficulty: current.Difficulty, creator: current.Creator, role: difficultyRole(currentStars, stars), stars: currentStars,
		details: fmt.Sprintf("AR %.1f     OD %.1f     CS %.1f     HP %.1f", d.GetAR(), d.GetOD(), d.GetCS(), d.GetHP()),
		timing:  fmt.Sprintf("%s     时长 %d:%02d     物件 %d", bpm, duration/60, duration%60, len(current.HitObjects)),
		end:     current.HitObjects[0].GetStartTime() - math.Min(1800, d.Preempt)}
}

// Hierarchical title/metadata and a difficulty accent follow osu!'s beatmap
// header convention; drawing uses danser's existing quad and font renderer.
func (overlay *KnockoutOverlay) drawBeatmapIntro(b *batch.QuadBatch) {
	info := overlay.intro
	if info == nil || settings.START > 0 || overlay.audioTime >= info.end {
		return
	}
	remaining := info.end - overlay.audioTime
	alpha := introCardAlpha(remaining)
	if alpha <= 0 {
		return
	}
	w := math.Min(1300, overlay.ScaledWidth*0.82)
	x := (overlay.ScaledWidth - w) / 2
	y := (overlay.ScaledHeight - 450) / 2
	y += 18 * (1 - alpha)
	rect := func(x, y, w, h, r, g, blue, a float64) {
		b.ResetTransform()
		b.SetColor(r, g, blue, a*alpha)
		b.SetSubScale(w/2, h/2)
		b.SetTranslation(vector.NewVec2d(x+w/2, y+h/2))
		b.DrawUnit(graphics.Pixel.GetRegion())
		b.ResetTransform()
	}
	rect(0, 0, overlay.ScaledWidth, overlay.ScaledHeight, 0, 0, 0, 0.65)
	rect(x+5, y+10, w, 450, 0, 0, 0, 0.35)
	rect(x, y, w, 450, 0.055, 0.045, 0.075, 0.98)
	rect(x, y, w, 5, 1, 0.4, 0.66, 1)
	rect(x+w-255, y+38, 207, 133, 0.18, 0.1, 0.2, 1)
	text := func(value string, yy, size, r, g, blue float64) {
		if width := overlay.font.GetWidth(size, value); width > w-360 {
			size *= (w - 360) / width
		}
		b.SetColor(r, g, blue, alpha)
		overlay.font.DrawOrigin(b, x+48, yy, vector.CentreLeft, size, false, value)
	}
	text("osu!standard  /  当前难度", y+42, 18, 0.95, 0.48, 0.7)
	b.SetColor(1, 0.76, 0.4, alpha)
	overlay.font.DrawOrigin(b, x+w-151, y+83, vector.Centre, 34, false, fmt.Sprintf("%.2f ★", info.stars))
	b.SetColor(0.8, 0.72, 0.86, alpha)
	overlay.font.DrawOrigin(b, x+w-151, y+123, vector.Centre, 15, false, "无 mod 星级")
	b.SetColor(0.65, 0.60, 0.72, alpha)
	overlay.font.DrawOrigin(b, x+w-151, y+151, vector.Centre, 13, false, "(rework前)")
	text(info.title, y+99, 44, 1, 1, 1)
	text(info.artist, y+151, 24, 0.75, 0.79, 0.86)
	name := "[" + info.difficulty + "]"
	if info.role != "" {
		name += "  ·  " + info.role
	}

	text(name, y+219, 30, 1, 0.75, 0.4)
	rect(x+48, y+258, w-96, 1, 0.4, 0.47, 0.58, 0.35)
	for i, part := range strings.Fields(info.details) {
		if i%2 != 0 {
			continue
		}
		fields := strings.Fields(info.details)
		bx := x + 48 + float64(i/2)*(w-96)/4
		rect(bx, y+280, (w-120)/4, 73, 0.1, 0.08, 0.13, 1)
		b.SetColor(0.7, 0.63, 0.77, alpha)
		overlay.font.DrawOrigin(b, bx+18, y+301, vector.CentreLeft, 15, false, part)
		b.SetColor(1, 1, 1, alpha)
		overlay.font.DrawOrigin(b, bx+18, y+331, vector.CentreLeft, 25, false, fields[i+1])
	}
	text(info.timing, y+383, 21, 0.8, 0.84, 0.9)
	text("制谱："+info.creator, y+422, 18, 0.65, 0.71, 0.8)
	b.ResetTransform()
	b.SetColor(1, 1, 1, 1)
}

// 450ms fade in, 3s fully visible, 450ms fade out, ending before gameplay.
func introCardAlpha(remaining float64) float64 {
	if remaining <= 0 || remaining >= 3900 {
		return 0
	}
	return math.Min(1, math.Min(remaining/450, (3900-remaining)/450))
}
