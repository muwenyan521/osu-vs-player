package osu

import "testing"

func TestStableStandardizedTailPreservesCombo(t *testing.T) {
	for _, combo := range []ComboResult{Increase, Hold} {
		original := JudgementResult{HitResult: SliderEnd, MaxResult: SliderEnd, ComboResult: combo}
		converted := stableStandardizedResult(original)
		if converted.HitResult != LegacySliderEnd || converted.MaxResult != LegacySliderEnd || converted.ComboResult != combo {
			t.Fatalf("incorrect converted tail: %+v", converted)
		}
		if original.HitResult != SliderEnd {
			t.Fatal("conversion modified original judgement")
		}
	}
	miss := stableStandardizedResult(JudgementResult{HitResult: SliderMiss, MaxResult: SliderEnd, ComboResult: Hold})
	if miss.HitResult != SliderMiss || miss.ComboResult != Hold || miss.MaxResult != LegacySliderEnd {
		t.Fatalf("miss semantics changed: %+v", miss)
	}
}

func TestStandardizedScoreIsNotClassicProjection(t *testing.T) {
	s := newScoreV3Processor(true)
	s.score = 869240
	s.basicHitCount = 1839
	if s.GetScore() == s.score {
		t.Fatal("test does not exercise classic projection")
	}
	if s.score != 869240 {
		t.Fatal("standardized accumulator must remain separate from display projection")
	}
}
