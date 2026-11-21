package dance

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wieku/danser-go/app/beatmap"
	"github.com/wieku/danser-go/app/graphics"
	"github.com/wieku/danser-go/app/rulesets/osu"
	"github.com/wieku/danser-go/app/settings"
)

// Pre-simulate the exact same judgement stream on an independent map/controller.
// Stored replay totals remain authoritative for awards; the elimination combo
// must come from the same simulation as the combo compared against it.
func (controller *ReplayController) prepareEliminationPeaks() {
	if len(controller.replays) <= 1 {
		return // Ordinary replay uses ScoreOverlay and has no elimination timeline.
	}
	if settings.Knockout.Mode != settings.MaxCombo && settings.Knockout.Mode != settings.ComboBreak {
		return
	}
	f, err := os.Open(filepath.Join(settings.General.GetSongsDir(), controller.bMap.Dir, controller.bMap.File))
	if err != nil {
		panic(err)
	}
	m := beatmap.ParseBeatMapFile(f)
	f.Close()
	if m == nil {
		panic("cannot parse pre-simulation beatmap")
	}
	m.MD5 = controller.bMap.MD5
	beatmap.ParseTimingPointsAndPauses(m)
	beatmap.ParseObjects(m, false, false)
	for _, o := range m.HitObjects {
		o.DisableAudioSubmission(true)
	}
	preview := NewReplayController().(*ReplayController)
	preview.preSimulating = true
	preview.SetBeatMap(m)
	preview.InitCursors()
	if len(preview.replays) != len(controller.replays) {
		panic("pre-simulation replay count changed")
	}

	segments := make(map[*graphics.Cursor][]comboBreakEvent)
	previous := make(map[*graphics.Cursor]int64)
	preview.ruleset.SetListener(func(cursor *graphics.Cursor, result osu.JudgementResult, score osu.Score) {
		if result.ComboResult == osu.Reset {
			segments[cursor] = append(segments[cursor], comboBreakEvent{previous[cursor], result.Time, result})
		}
		previous[cursor] = int64(score.CurrentCombo)
	})
	end := int64(m.HitObjects[len(m.HitObjects)-1].GetEndTime()) + 5000
	for time := int64(-1000); time <= end; time++ {
		preview.updateMain(float64(time))
	}
	for i, r := range preview.replays {
		if r.Name != controller.replays[i].Name {
			panic("pre-simulation replay order changed")
		}
		score := preview.ruleset.GetScore(preview.cursors[i])
		peak := int64(score.Combo)
		when := eliminationMissTime(segments[preview.cursors[i]], peak, int64(score.CurrentCombo))
		controller.replays[i].EliminationCombo = peak
		controller.replays[i].EliminationTime = when
		log.Printf("Elimination peak %s: stored=%d simulated=%d break=%d", r.RawName, r.ExpectedMaxCombo, peak, when)
	}
	log.Println(fmt.Sprintf("Pre-simulated elimination peaks for %d replays", len(preview.replays)))
}

// IsMaxComboMiss excludes slider breaks and tails, even when they reset combo.
func IsMaxComboMiss(result osu.JudgementResult, combo, peak int64) bool {
	return peak > 0 && combo >= peak && result.ComboResult == osu.Reset &&
		result.HitResult&osu.BaseHitsM == osu.Miss &&
		result.MaxResult&(osu.SliderEnd|osu.LegacySliderEnd) == 0
}

type comboBreakEvent struct {
	combo  int64
	time   int64
	result osu.JudgementResult
}

// A terminal longest combo uses the last real miss. Otherwise, eliminate at
// the first real miss at or after a longest combo breaks. No miss means no elimination.
func eliminationMissTime(events []comboBreakEvent, peak, finalCombo int64) int64 {
	if peak <= 0 {
		return -1
	}
	if finalCombo == peak {
		for i := len(events) - 1; i >= 0; i-- {
			if IsMaxComboMiss(events[i].result, peak, peak) {
				return events[i].time
			}
		}
		return -1
	}
	for i, event := range events {
		if event.combo < peak {
			continue
		}
		// Once a longest combo ends, slider breaks defer elimination until
		// the first genuine miss. A genuine miss ends it immediately.
		for _, next := range events[i:] {
			if IsMaxComboMiss(next.result, peak, peak) {
				return next.time
			}
		}
		return -1
	}
	return -1
}
