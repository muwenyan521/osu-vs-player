package osu

import "testing"

func TestApplyReplayErrorFix(t *testing.T) {
	result := JudgementResult{HitResult: PositionalMiss, ComboResult: Reset}
	fix := ReplayErrorFix{Result: 1, IsBreak: false}

	if !applyReplayErrorFix(fix, &result) {
		t.Fatal("valid replay correction was rejected")
	}
	if result.HitResult != Hit100 || result.ComboResult != Increase {
		t.Fatalf("corrected result = (%v, %v), want (%v, %v)", result.HitResult, result.ComboResult, Hit100, Increase)
	}
}

func TestApplyReplayErrorFixRejectsInvalidResult(t *testing.T) {
	result := JudgementResult{HitResult: Miss, ComboResult: Reset}
	if applyReplayErrorFix(ReplayErrorFix{Result: 4}, &result) {
		t.Fatal("invalid replay correction was accepted")
	}
	if result.HitResult != Miss || result.ComboResult != Reset {
		t.Fatalf("invalid correction changed result: %+v", result)
	}
}
