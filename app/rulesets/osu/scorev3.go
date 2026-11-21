package osu

import (
	"github.com/wieku/danser-go/app/beatmap"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/app/settings"
	"math"
)

type scoreV3Processor struct {
	score    int64
	combo    int64
	accuracy float64

	modMultiplier float64

	bonus int64

	comboPart    float64
	comboPartMax float64

	accPart    int64
	accPartMax int64

	hits          int64
	maxHits       int64
	basicHitCount int64

	player *difficultyPlayer

	forceClassic bool
}

func newScoreV3Processor(forceClassic bool) *scoreV3Processor {
	return &scoreV3Processor{
		forceClassic: forceClassic,
	}
}

func (s *scoreV3Processor) Init(beatMap *beatmap.BeatMap, player *difficultyPlayer) {
	s.player = player
	s.modMultiplier = player.diff.GetScoreMultiplier()

	s.comboPartMax = 0
	s.maxHits = 0

	for _, o := range beatMap.HitObjects {
		if o.GetType() == objects.SPINNER {
			s.AddResult(createJudgementResult(Hit300, Hit300, Increase, int64(o.GetEndTime()), o.GetStartPosition(), nil))
		} else if slider, ok := o.(*objects.Slider); ok {
			if player.lzNoSliderAcc {
				s.AddResult(createJudgementResult(SliderStart, SliderStart, Increase, int64(o.GetStartTime()), o.GetStartPosition(), nil))
			} else {
				s.AddResult(createJudgementResult(Hit300, Hit300, Increase, int64(o.GetStartTime()), o.GetStartPosition(), nil))
			}

			for i, p := range slider.ScorePointsLazer {
				if i == len(slider.ScorePoints)-1 {
					if player.lzNoSliderAcc {
						s.AddResult(createJudgementResult(LegacySliderEnd, LegacySliderEnd, Hold, int64(p.Time), p.Pos, nil))
					} else {
						s.AddResult(createJudgementResult(SliderEnd, SliderEnd, Increase, int64(p.Time), p.Pos, nil))
					}
				} else if p.IsReverse {
					s.AddResult(createJudgementResult(SliderRepeat, SliderRepeat, Increase, int64(p.Time), p.Pos, nil))
				} else {
					s.AddResult(createJudgementResult(SliderPoint, SliderPoint, Increase, int64(p.Time), p.Pos, nil))
				}
			}

			if player.lzNoSliderAcc {
				s.AddResult(createJudgementResult(Hit300, Hit300, Increase, int64(o.GetEndTime()), o.GetStartPosition(), nil))
			}
		} else {
			s.AddResult(createJudgementResult(Hit300, Hit300, Increase, int64(o.GetStartTime()), o.GetStartPosition(), nil))
		}
	}

	s.comboPartMax = s.comboPart
	s.maxHits = s.hits

	s.combo = 0
	s.hits = 0
	s.comboPart = 0
	s.bonus = 0
	s.accPart = 0
	s.accPartMax = 0
	s.accuracy = 1
	s.basicHitCount = 0
}

func (s *scoreV3Processor) AddResult(result JudgementResult) {
	if result.ComboResult == Reset || result.HitResult == Miss {
		s.combo = 0
	} else if result.ComboResult == Increase {
		s.combo++
	}

	if result.HitResult.IsBonus() {
		s.bonus += result.HitResult.ScoreValueLazer()
	} else if result.HitResult.AffectsAccLZ() {
		s.accPart += result.HitResult.ScoreValueLazer()
		s.accPartMax += result.MaxResult.ScoreValueLazer()

		s.hits++
	}

	if result.HitResult&BaseHitsM > 0 {
		s.basicHitCount++
	}

	// slider end misses (not classic mod!) don't propagate combo score
	if result.HitResult.AffectsAccLZ() && !(result.HitResult == SliderMiss && result.MaxResult == SliderEnd) {
		s.comboPart += float64(result.MaxResult.ScoreValueLazer()) * math.Pow(float64(s.combo), 0.5)
	}

	if s.maxHits == 0 {
		return
	}

	acc := 1.0
	if s.hits > 0 && s.accPartMax > 0 {
		acc = float64(s.accPart) / float64(s.accPartMax)
	}

	s.accuracy = acc

	comboProgress := s.comboPart / s.comboPartMax
	accProgress := float64(s.hits) / float64(s.maxHits)

	s.score = int64(math.Round(math.Round(500000*acc*comboProgress+500000*math.Pow(acc, 5)*accProgress+float64(s.bonus)) * s.modMultiplier))
}

func (s *scoreV3Processor) ModifyResult(result HitResult, src HitObject) HitResult {
	return result
}

func (s *scoreV3Processor) GetScore() int64 {
	if settings.Gameplay.LazerClassicScore || s.forceClassic {
		return int64(math.Round((math.Pow(float64(s.basicHitCount), 2)*32.57 + 100000) * float64(s.score) / 1000000))
	}

	return s.score
}

func (s *scoreV3Processor) GetCombo() int64 {
	return s.combo
}

func (s *scoreV3Processor) GetAccuracy() float64 {
	return s.accuracy
}

// initStable uses the stable judgement stream and its original combo semantics,
// but accumulates the same accuracy/combo/bonus standard-score components as V3.
// It does not change hit windows, note lock, HP, PP, or the original score.
func (s *scoreV3Processor) initStable(beatMap *beatmap.BeatMap, player *difficultyPlayer) {
	s.player = player
	s.modMultiplier = player.diff.GetScoreMultiplierV2()
	for _, object := range beatMap.HitObjects {
		if slider, ok := object.(*objects.Slider); ok {
			s.AddResult(JudgementResult{HitResult: SliderStart, MaxResult: SliderStart, ComboResult: Increase})
			for i, point := range slider.ScorePoints {
				result := SliderPoint
				if i == len(slider.ScorePoints)-1 {
					result = LegacySliderEnd
				} else if point.IsReverse {
					result = SliderRepeat
				}
				s.AddResult(JudgementResult{HitResult: result, MaxResult: result, ComboResult: Increase})
			}
			s.AddResult(JudgementResult{HitResult: Hit300, MaxResult: Hit300, ComboResult: Hold})
		} else {
			s.AddResult(JudgementResult{HitResult: Hit300, MaxResult: Hit300, ComboResult: Increase})
		}
	}
	s.comboPartMax, s.maxHits = s.comboPart, s.hits
	s.score, s.combo, s.bonus, s.accPart, s.accPartMax, s.hits, s.basicHitCount = 0, 0, 0, 0, 0, 0, 0
	s.comboPart, s.accuracy = 0, 1
}

func stableStandardizedResult(result JudgementResult) JudgementResult {
	if result.MaxResult&SliderEnd != 0 {
		result.MaxResult = LegacySliderEnd
		if result.HitResult&SliderEnd != 0 {
			result.HitResult = LegacySliderEnd
		}
	}
	return result
}
