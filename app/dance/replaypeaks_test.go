package dance

import (
	"github.com/wieku/danser-go/app/rulesets/osu"
	"testing"
)

func TestMaxComboEliminationRequiresTrueMiss(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result osu.JudgementResult
		combo  int64
		want   bool
	}{
		{"miss at peak", osu.JudgementResult{HitResult: osu.Miss, MaxResult: osu.Hit300, ComboResult: osu.Reset}, 100, true},
		{"miss before peak", osu.JudgementResult{HitResult: osu.Miss, MaxResult: osu.Hit300, ComboResult: osu.Reset}, 99, false},
		{"slider break at peak", osu.JudgementResult{HitResult: osu.SliderMiss, MaxResult: osu.SliderPoint, ComboResult: osu.Reset}, 100, false},
		{"lazer tail", osu.JudgementResult{HitResult: osu.Miss, MaxResult: osu.SliderEnd, ComboResult: osu.Reset}, 100, false},
		{"stable tail", osu.JudgementResult{HitResult: osu.Miss, MaxResult: osu.LegacySliderEnd, ComboResult: osu.Reset}, 100, false},
		{"positional miss", osu.JudgementResult{HitResult: osu.PositionalMiss, MaxResult: osu.Hit300, ComboResult: osu.Reset}, 100, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsMaxComboMiss(tc.result, tc.combo, 100); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEliminationMissTime(t *testing.T) {
	miss := osu.JudgementResult{HitResult: osu.Miss, MaxResult: osu.Hit300, ComboResult: osu.Reset}
	slider := osu.JudgementResult{HitResult: osu.SliderMiss, MaxResult: osu.SliderPoint, ComboResult: osu.Reset}
	for _, tc := range []struct {
		name       string
		events     []comboBreakEvent
		finalCombo int64
		want       int64
	}{
		{"terminal peak uses last miss", []comboBreakEvent{{20, 100, miss}, {40, 200, miss}}, 100, 200},
		{"terminal tied peak overrides earlier peak miss", []comboBreakEvent{{100, 100, miss}, {40, 200, miss}}, 100, 200},
		{"terminal peak ignores later slider break", []comboBreakEvent{{20, 100, miss}, {40, 200, slider}}, 100, 100},
		{"zero miss survives", []comboBreakEvent{{40, 200, slider}}, 100, -1},
		{"ordinary peak miss", []comboBreakEvent{{100, 100, miss}, {40, 200, miss}}, 20, 100},
		{"no miss after peak slider break", []comboBreakEvent{{20, 100, miss}, {100, 200, slider}}, 20, -1},
		{"earlier misses do not qualify after slider break", []comboBreakEvent{{20, 100, miss}, {40, 150, miss}, {100, 200, slider}}, 20, -1},
		{"peak slider break uses first later miss", []comboBreakEvent{{100, 100, slider}, {40, 200, miss}, {30, 300, miss}}, 20, 200},
		{"slider break not last reset", []comboBreakEvent{{20, 100, miss}, {100, 200, slider}, {30, 300, slider}}, 20, -1},
		{"later slider break ignored before first miss", []comboBreakEvent{{100, 100, slider}, {40, 200, slider}, {30, 300, miss}}, 20, 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := eliminationMissTime(tc.events, 100, tc.finalCombo); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}
