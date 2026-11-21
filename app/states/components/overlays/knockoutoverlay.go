package overlays

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/app/dance"
	"github.com/wieku/danser-go/app/discord"
	"github.com/wieku/danser-go/app/graphics"
	"github.com/wieku/danser-go/app/rulesets/osu"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/app/skin"
	"github.com/wieku/danser-go/app/utils"
	"github.com/wieku/danser-go/framework/assets"
	"github.com/wieku/danser-go/framework/bass"
	"github.com/wieku/danser-go/framework/graphics/batch"
	"github.com/wieku/danser-go/framework/graphics/font"
	"github.com/wieku/danser-go/framework/graphics/texture"
	"github.com/wieku/danser-go/framework/math/animation"
	"github.com/wieku/danser-go/framework/math/animation/easing"
	color2 "github.com/wieku/danser-go/framework/math/color"
	"github.com/wieku/danser-go/framework/math/vector"
)

type stats struct {
	pp       float64
	score    int64
	accuracy float64
}

type finalStanding struct {
	name       string
	colorIndex int
	awards     []string
	group      string
}

type knockoutPlayer struct {
	fade   *animation.Glider
	slide  *animation.Glider
	height *animation.Glider
	rowY   *animation.TargetGlider
	index  *animation.Glider

	scoreDisp *animation.TargetGlider
	ppDisp    *animation.TargetGlider
	accDisp   *animation.TargetGlider

	lastCombo      int64
	sCombo         int64
	maxCombo       int64
	hasBroken      bool
	protectedBreak bool
	breakTime      int64
	pp             float64
	score          int64
	ur             float64
	hitErrorMean   float64
	hitErrorM2     float64
	hitErrorCount  int

	perObjectStats []stats

	displayHp float64

	lastHit  osu.HitResult
	fadeHit  *animation.Glider
	scaleHit *animation.Glider

	name         string
	oldIndex     int
	currentIndex int
}

type bubble struct {
	deathFade  *animation.Glider
	deathSlide *animation.Glider
	deathX     float64
	endTime    float64
	name       string
	combo      int64
	lastHit    osu.HitResult
	lastCombo  osu.ComboResult
	deathScale *animation.Glider
}

func newBubble(position vector.Vector2f, time float64, name string, combo int64, lastHit osu.HitResult, lastCombo osu.ComboResult) *bubble {
	deathShiftX := (rand.Float64() - 0.5) * 10
	deathShiftY := (rand.Float64() - 0.5) * 10
	baseY := position.Y64() + deathShiftY

	bub := new(bubble)
	bub.name = name
	bub.deathX = position.X64() + deathShiftX
	bub.deathSlide = animation.NewGlider(0.0)
	bub.deathFade = animation.NewGlider(0.0)
	bub.deathScale = animation.NewGlider(1)
	bub.deathSlide.SetEasing(easing.OutQuad)

	if settings.Knockout.Mode == settings.OneVsOne {
		bub.deathSlide.AddEventS(time, time+2000, baseY, baseY)
		bub.deathFade.AddEventS(time, time+difficulty.ResultFadeIn, 0, 1)
		bub.deathFade.AddEventS(time+difficulty.PostEmpt, time+difficulty.PostEmpt+difficulty.ResultFadeOut, 1, 0)
		bub.deathScale.AddEventSEase(time, time+difficulty.ResultFadeIn*1.2, 0.4, 1, easing.OutElastic)
	} else {
		bub.deathSlide.AddEventS(time, time+2000, baseY, baseY+50)
		bub.deathFade.AddEventS(time, time+200, 0, 1)
		bub.deathFade.AddEventS(time+800, time+1200, 1, 0)
	}

	bub.endTime = time + 2000
	bub.combo = combo
	bub.lastHit = lastHit
	bub.lastCombo = lastCombo

	return bub
}

type KnockoutOverlay struct {
	controller   *dance.ReplayController
	font         *font.Font
	players      map[string]*knockoutPlayer
	playersArray []*knockoutPlayer
	deathBubbles []*bubble
	names        map[*graphics.Cursor]string
	generator    *rand.Rand

	audioTime  float64
	normalTime float64

	Button        *texture.TextureRegion
	ButtonClicked *texture.TextureRegion

	ScaledHeight float64
	ScaledWidth  float64

	music bass.ITrack

	breakMode bool
	fade      *animation.Glider

	alivePlayers int

	resultsStart   float64
	resultsReady   bool
	finalStandings []finalStanding
	playerPalette  []color2.Color
	intro          *beatmapIntro
}

func NewKnockoutOverlay(replayController *dance.ReplayController) *KnockoutOverlay {
	overlay := new(KnockoutOverlay)
	overlay.controller = replayController
	overlay.intro = loadBeatmapIntro(replayController.GetBeatMap())
	overlay.playerPalette = distinctPlayerPalette(len(replayController.GetReplays()))

	if font.GetFont("Quicksand Bold") == nil {
		file, _ := assets.Open("assets/fonts/Quicksand-Bold.ttf")
		font.LoadFont(file)
		file.Close()
	}
	overlay.font = font.GetFont("Quicksand Bold")
	if strings.TrimSpace(settings.Gameplay.HUDFont) != "" {
		characters := " · ：、/0123456789赛后荣誉榜玩家保留原因与最终数据暂无回放成绩最高分数最高连击最少失误并列最高准确率续上页"
		for _, replay := range replayController.GetReplays() {
			characters += replay.RawName + replay.Name + replay.Mods
		}
		characters += "非榜" + settings.Knockout.Watermark
		characters += "当前难度主次无星级时长物件制谱星·–★前" + overlay.intro.title + overlay.intro.artist + overlay.intro.difficulty + overlay.intro.creator
		loadSubFont(settings.Gameplay.HUDFont, "KnockoutFont", characters)
		if configured := font.GetFont("KnockoutFont"); configured != nil {
			overlay.font = configured
		}
	}

	overlay.players = make(map[string]*knockoutPlayer)
	overlay.playersArray = make([]*knockoutPlayer, 0)
	overlay.deathBubbles = make([]*bubble, 0)
	overlay.names = make(map[*graphics.Cursor]string)
	overlay.generator = rand.New(rand.NewSource(replayController.GetBeatMap().TimeAdded))

	overlay.ScaledHeight = 1080.0
	overlay.ScaledWidth = overlay.ScaledHeight * settings.Graphics.GetAspectRatio()

	overlay.fade = animation.NewGlider(1)

	for i, r := range replayController.GetReplays() {
		cursor := replayController.GetCursors()[i]
		overlay.names[cursor] = r.Name
		overlay.players[r.Name] = &knockoutPlayer{
			fade:           animation.NewGlider(1),
			slide:          animation.NewGlider(0),
			height:         animation.NewGlider(overlay.ScaledHeight * 0.9 * 1.2 / float64(max(51, settings.PLAYERS+1))),
			rowY:           animation.NewTargetGlider(float64(i)*overlay.ScaledHeight*0.9*1.2/float64(max(51, settings.PLAYERS+1)), 2),
			index:          animation.NewGlider(float64(i)),
			scoreDisp:      animation.NewTargetGlider(0, 0),
			ppDisp:         animation.NewTargetGlider(0, 2),
			accDisp:        animation.NewTargetGlider(100, 2),
			maxCombo:       r.EliminationCombo,
			perObjectStats: make([]stats, len(replayController.GetBeatMap().HitObjects)),
			lastHit:        osu.Hit300,
			fadeHit:        animation.NewGlider(0),
			scaleHit:       animation.NewGlider(0),
			name:           r.Name,
			oldIndex:       i,
			currentIndex:   i,
		}
		overlay.players[r.Name].index.SetEasing(easing.InOutQuad)
		overlay.playersArray = append(overlay.playersArray, overlay.players[r.Name])

		overlay.alivePlayers++
	}

	if settings.Knockout.LiveSort {
		rand.Shuffle(len(overlay.playersArray), func(i, j int) {
			overlay.playersArray[i], overlay.playersArray[j] = overlay.playersArray[j], overlay.playersArray[i]
		})
	}

	discord.UpdateKnockout(len(overlay.playersArray), len(overlay.playersArray))

	for i, g := range overlay.playersArray {
		if i != g.currentIndex {
			g.index.Reset()
			g.index.SetValue(float64(i))
			g.currentIndex = i
		}
	}

	replayController.GetRuleset().SetListener(overlay.hitReceived)

	sortFunc := func(number int64, instantSort bool) {
		alive := 0
		for _, g := range overlay.playersArray {
			if !g.hasBroken {
				alive++
			}
		}

		if settings.Knockout.LiveSort {
			cond := strings.ToLower(settings.Knockout.SortBy)

			sort.SliceStable(overlay.playersArray, func(i, j int) bool {
				mainCond := true
				switch cond {
				case "pp":
					mainCond = overlay.playersArray[i].perObjectStats[number].pp > overlay.playersArray[j].perObjectStats[number].pp
				case "acc", "accuracy":
					mainCond = overlay.playersArray[i].perObjectStats[number].accuracy > overlay.playersArray[j].perObjectStats[number].accuracy
				default:
					mainCond = overlay.playersArray[i].perObjectStats[number].score > overlay.playersArray[j].perObjectStats[number].score
				}

				return (!overlay.playersArray[i].hasBroken && overlay.playersArray[j].hasBroken) || ((!overlay.playersArray[i].hasBroken && !overlay.playersArray[j].hasBroken) && mainCond) || ((overlay.playersArray[i].hasBroken && overlay.playersArray[j].hasBroken) && (overlay.playersArray[i].breakTime > overlay.playersArray[j].breakTime || (overlay.playersArray[i].breakTime == overlay.playersArray[j].breakTime && mainCond)))
			})

			for i, g := range overlay.playersArray {
				if i != g.currentIndex {
					g.index.Reset()

					animDuration := 0.0
					if !instantSort {
						animDuration = 200 + math.Abs(float64(i-g.currentIndex))*10
					}

					g.index.AddEvent(overlay.normalTime, overlay.normalTime+animDuration, float64(i))
					g.currentIndex = i
				}
			}
		}

		discord.UpdateKnockout(alive, len(overlay.playersArray))
	}

	replayController.GetRuleset().SetEndListener(func(time int64, number int64) {
		if number == int64(len(replayController.GetBeatMap().HitObjects)-1) && settings.Knockout.RevivePlayersAtEnd {
			for _, player := range overlay.players {
				player.hasBroken = false
				player.protectedBreak = false
				player.breakTime = 0

				player.fade.Reset()
				player.fade.AddEvent(overlay.normalTime, overlay.normalTime+750, 1)

				player.height.Reset()
				player.height.SetEasing(easing.InQuad)
				player.height.AddEvent(overlay.normalTime, overlay.normalTime+200, overlay.ScaledHeight*0.9*1.2/float64(max(51, settings.PLAYERS+1)))
			}

			sortFunc(number, true)
		} else {
			sortFunc(number, false)
		}
	})

	overlay.Button = skin.GetTexture("knockout-button")
	overlay.ButtonClicked = skin.GetTexture("knockout-button-active")

	return overlay
}

func parseKnockoutPlayerColors(spec string) map[string]color2.Color {
	parsed := make(map[string]color2.Color)
	for _, entry := range strings.Split(spec, "|") {
		name, rawRGB, ok := strings.Cut(strings.TrimSpace(entry), ":")
		if !ok || name == "" {
			continue
		}

		channels := strings.Split(rawRGB, ",")
		if len(channels) != 3 {
			continue
		}

		var rgb [3]float32
		valid := true
		for i, channel := range channels {
			value, err := strconv.Atoi(strings.TrimSpace(channel))
			if err != nil || value < 0 || value > 255 {
				valid = false
				break
			}
			rgb[i] = float32(value) / 255
		}
		if valid {
			parsed[name] = color2.NewRGBA(rgb[0], rgb[1], rgb[2], 1)
		}
	}
	return parsed
}

func (overlay *KnockoutOverlay) ApplyCursorColors(colors []color2.Color) {
	custom := parseKnockoutPlayerColors(settings.Knockout.PlayerColors)
	replays := overlay.controller.GetReplays()
	for playerIndex, replay := range replays {
		c, exists := custom[replay.Name]
		if !exists {
			c, exists = custom[replay.RawName]
		}
		if !exists && settings.Knockout.DistinctPlayerColors && playerIndex < len(overlay.playerPalette) {
			c, exists = overlay.playerPalette[playerIndex], true
		}
		if !exists {
			continue
		}
		for divide := 0; divide < settings.DIVIDES; divide++ {
			index := divide*len(replays) + playerIndex
			if index < len(colors) {
				c.A = colors[index].A
				colors[index] = c
			}
		}
	}
}

func replayFinalAccuracy(replay dance.RpData) float64 {
	total := replay.Expected300 + replay.Expected100 + replay.Expected50 + replay.ExpectedMiss
	if total == 0 {
		return 0
	}
	return float64(replay.Expected300*300+replay.Expected100*100+replay.Expected50*50) / float64(total*300)
}

func isEasyHalfTime(mods difficulty.Modifier) bool {
	return mods&(difficulty.Easy|difficulty.HalfTime|difficulty.Daycore) != 0
}

func replayLeaderProtected(candidate dance.RpData, replays []dance.RpData) bool {
	if settings.Knockout.SplitEasyHalfTime {
		filtered := make([]dance.RpData, 0, len(replays))
		for _, r := range replays {
			if isEasyHalfTime(r.ModsV) == isEasyHalfTime(candidate.ModsV) {
				filtered = append(filtered, r)
			}
		}
		replays = filtered
	}
	if !candidate.HasExpected {
		return false
	}

	var highestScore int64
	highestPP := 0.0
	highestAccuracy := 0.0
	var highestMaxCombo int64
	var fewestMisses uint
	hasReplay := false
	hasPerfectReplay := false

	for _, replay := range replays {
		if !replay.HasExpected {
			continue
		}
		hasReplay = true
		highestScore = max(highestScore, replay.ExpectedScore)
		highestPP = max(highestPP, replay.ExpectedPP)
		highestAccuracy = max(highestAccuracy, replayFinalAccuracy(replay))
		highestMaxCombo = max(highestMaxCombo, replay.ExpectedMaxCombo)
		if replay.ExpectedMiss == 0 {
			hasPerfectReplay = true
		}
		if fewestMisses == 0 || replay.ExpectedMiss < fewestMisses {
			fewestMisses = replay.ExpectedMiss
		}
	}

	if !hasReplay {
		return false
	}
	awards := []func(dance.RpData) bool{
		func(r dance.RpData) bool {
			return highestAccuracy > 0 && math.Abs(replayFinalAccuracy(r)-highestAccuracy) < 1e-12
		},
		func(r dance.RpData) bool { return highestScore > 0 && r.ExpectedScore == highestScore },
		func(r dance.RpData) bool { return highestPP > 0 && math.Abs(r.ExpectedPP-highestPP) < 1e-6 },
		func(r dance.RpData) bool { return highestMaxCombo > 0 && r.ExpectedMaxCombo == highestMaxCombo },
		func(r dance.RpData) bool {
			return !hasPerfectReplay && fewestMisses > 0 && r.ExpectedMiss == fewestMisses
		},
	}
	for _, wins := range awards {
		if !wins(candidate) {
			continue
		}
		count := 0
		for _, r := range replays {
			if r.HasExpected && wins(r) {
				count++
			}
		}
		if count <= 5 {
			return true
		}
	}
	return false
}

func brokeRecordedMaxCombo(currentCombo, recordedMaxCombo int64) bool {
	return recordedMaxCombo > 0 && currentCombo >= recordedMaxCombo
}

func buildFinalStandings(replays []dance.RpData) []finalStanding {
	if !settings.Knockout.SplitEasyHalfTime {
		return buildFinalStandingsGroup(replays)
	}
	result := []finalStanding{}
	for _, group := range []bool{false, true} {
		subset := []dance.RpData{}
		indexes := []int{}
		for i, r := range replays {
			if isEasyHalfTime(r.ModsV) == group {
				subset = append(subset, r)
				indexes = append(indexes, i)
			}
		}
		rows := buildFinalStandingsGroup(subset)
		for i := range rows {
			rows[i].colorIndex = indexes[rows[i].colorIndex]
			if group {
				rows[i].group = "HT / EZ 榜"
			} else {
				rows[i].group = "非 HT / EZ 榜"
			}
		}
		result = append(result, rows...)
	}
	return result
}

func buildFinalStandingsGroup(replays []dance.RpData) []finalStanding {
	var highestScore int64
	highestPP := 0.0
	highestAccuracy := 0.0
	var highestCombo int64
	var fewestMisses uint
	hasReplay := false

	for _, replay := range replays {
		if !replay.HasExpected {
			continue
		}
		highestScore = max(highestScore, replay.ExpectedScore)
		highestPP = max(highestPP, replay.ExpectedPP)
		highestAccuracy = max(highestAccuracy, replayFinalAccuracy(replay))
		highestCombo = max(highestCombo, replay.ExpectedMaxCombo)
		if !hasReplay || replay.ExpectedMiss < fewestMisses {
			fewestMisses = replay.ExpectedMiss
		}
		hasReplay = true
	}
	if !hasReplay {
		return nil
	}

	standings := make([]finalStanding, 0)
	indexes := make(map[string]int)
	addAward := func(replay dance.RpData, colorIndex int, award string) {
		key := strings.ToLower(strings.TrimSpace(replay.RawName))
		if key == "" {
			key = replay.Name
		}
		if index, ok := indexes[key]; ok {
			standings[index].awards = append(standings[index].awards, award)
			return
		}
		name := replay.RawName
		if name == "" {
			name = replay.Name
		}
		if settings.Knockout.SplitEasyHalfTime {
			mods := (replay.ModsV & ^difficulty.Lazer).String()
			if mods != "" {
				name += " +" + mods
			}
		}
		indexes[key] = len(standings)
		standings = append(standings, finalStanding{
			name:       name,
			colorIndex: colorIndex,
			awards:     []string{award},
		})
	}

	for i, replay := range replays {
		if !replay.HasExpected {
			continue
		}
		if highestAccuracy > 0 && math.Abs(replayFinalAccuracy(replay)-highestAccuracy) < 1e-12 {
			addAward(replay, i, fmt.Sprintf("最高准确率：%.4f%%", replayFinalAccuracy(replay)*100))
		}
		if highestScore > 0 && replay.ExpectedScore == highestScore {
			addAward(replay, i, "最高分数："+utils.Humanize(replay.ExpectedScore))
		}
		if highestPP > 0 && math.Abs(replay.ExpectedPP-highestPP) < 1e-6 {
			addAward(replay, i, fmt.Sprintf("最高 PP：%.2f", replay.ExpectedPP))
		}
		if highestCombo > 0 && replay.ExpectedMaxCombo == highestCombo {
			addAward(replay, i, fmt.Sprintf("最高连击：%dx", replay.ExpectedMaxCombo))
		}
		if fewestMisses > 0 && replay.ExpectedMiss == fewestMisses {
			addAward(replay, i, fmt.Sprintf("最少失误：%d miss", replay.ExpectedMiss))
		}
	}

	return standings
}

func (overlay *KnockoutOverlay) SetResultsStart(time float64) {
	overlay.resultsStart = time
	overlay.finalStandings = buildFinalStandings(overlay.controller.GetReplays())
	overlay.resultsReady = true
}

func updatePlayerUR(player *knockoutPlayer, hitError, speed float64) {
	if speed <= 0 {
		return
	}

	player.hitErrorCount++
	delta := hitError - player.hitErrorMean
	player.hitErrorMean += delta / float64(player.hitErrorCount)
	player.hitErrorM2 += delta * (hitError - player.hitErrorMean)
	variance := max(0, player.hitErrorM2/float64(player.hitErrorCount))
	player.ur = 10 * math.Sqrt(variance) / speed
}

func (overlay *KnockoutOverlay) hitReceived(cursor *graphics.Cursor, judgementResult osu.JudgementResult, score osu.Score) {
	if judgementResult.HitResult == osu.PositionalMiss {
		return
	}

	player := overlay.players[overlay.names[cursor]]

	if cursor.InvertDisplayH {
		judgementResult.Position.X = 512 - judgementResult.Position.X
	}
	if cursor.InvertDisplayV {
		judgementResult.Position.Y = 384 - judgementResult.Position.Y
	}

	player.score = score.Score
	if settings.Knockout.ReplayTotalScoresFile != "" {
		player.score = score.StandardizedScore
	}
	player.pp = score.PP.Total

	if judgementResult.Number >= 0 && int(judgementResult.Number) < len(overlay.controller.GetRuleset().GetBeatMap().HitObjects) && judgementResult.HitResult&osu.BaseHitsM > 0 && judgementResult.HitResult&osu.BaseHitsM != osu.Miss {
		object := overlay.controller.GetRuleset().GetBeatMap().HitObjects[judgementResult.Number]
		_, circle := object.(*objects.Circle)
		_, slider := object.(*objects.Slider)
		if circle || slider {
			updatePlayerUR(player, float64(judgementResult.Time)-object.GetStartTime(), overlay.controller.GetRuleset().GetPlayerDifficulty(cursor).Speed)
		}
	}

	player.scoreDisp.SetValue(float64(player.score), false)
	player.ppDisp.SetValue(player.pp, false)

	if settings.Knockout.ReplayTotalScoresFile != "" && judgementResult.Number == int64(len(overlay.controller.GetBeatMap().HitObjects)-1) && judgementResult.HitResult&osu.BaseHitsM != 0 {
		player.score = overlay.controller.GetReplays()[player.oldIndex].ExpectedScore
		player.scoreDisp.SetValue(float64(player.score), false)
	}
	player.perObjectStats[judgementResult.Number].score = player.score
	player.perObjectStats[judgementResult.Number].pp = player.pp
	player.perObjectStats[judgementResult.Number].accuracy = score.Accuracy

	player.accDisp.SetValue(score.Accuracy*100, false)

	if judgementResult.ComboResult == osu.Increase {
		player.sCombo++
	}

	resultClean := judgementResult.HitResult & osu.BaseHitsM

	acceptableHits := !isSliderTailResult(judgementResult) && resultClean&(osu.Hit100|osu.Hit50|osu.Miss) > 0
	if acceptableHits {
		player.fadeHit.Reset()
		player.fadeHit.AddEventS(overlay.normalTime, overlay.normalTime+300, 0.5, 1)
		player.fadeHit.AddEventS(overlay.normalTime+600, overlay.normalTime+900, 1, 0)
		player.scaleHit.AddEventS(overlay.normalTime, overlay.normalTime+300, 0.5, 1)
		player.lastHit = judgementResult.HitResult & (osu.HitValues | osu.Miss)
	}
	if !isSliderTailResult(judgementResult) && shouldShowKnockoutMiss(resultClean, player.hasBroken) {
		overlay.deathBubbles = append(overlay.deathBubbles, newBubble(judgementResult.Position, overlay.normalTime, overlay.names[cursor], player.sCombo, resultClean, judgementResult.ComboResult))
	}

	comboBreak := judgementResult.ComboResult == osu.Reset
	maxComboMode := settings.Knockout.Mode == settings.MaxCombo || settings.Knockout.Mode == settings.ComboBreak
	if !maxComboMode && ((settings.Knockout.Mode == settings.SSOrQuit && (acceptableHits || comboBreak)) || (comboBreak && judgementResult.Number != 0)) {
		if !player.hasBroken {
			if settings.Knockout.Mode == settings.XReplays {
				if player.sCombo >= int64(settings.Knockout.BubbleMinimumCombo) {
					log.Println(overlay.names[cursor], "has broken! Combo:", player.sCombo)
				}
			} else if settings.Knockout.Mode == settings.SSOrQuit && overlay.alivePlayers > settings.Knockout.MinPlayers {
				player.hasBroken = true
				player.breakTime = judgementResult.Time
				overlay.alivePlayers--
				player.fade.AddEvent(overlay.normalTime, overlay.normalTime+500, 0)
				player.height.SetEasing(easing.OutQuad)
				player.height.AddEvent(overlay.normalTime+500, overlay.normalTime+850, 0)
				log.Println(overlay.names[cursor], "has broken! Combo:", player.sCombo)
			}
		}
	}

	if comboBreak {
		player.sCombo = 0
	}
}

func (overlay *KnockoutOverlay) Update(time float64) {
	if overlay.audioTime == 0 {
		overlay.audioTime = time
		overlay.normalTime = time
	}

	delta := time - overlay.audioTime

	if overlay.music != nil && overlay.music.GetState() == bass.MusicPlaying {
		delta /= overlay.music.GetSpeed()
	}

	overlay.normalTime += delta

	overlay.audioTime = time

	overlay.updateBreaks(overlay.normalTime)
	overlay.fade.Update(overlay.normalTime)

	for _, r := range overlay.controller.GetReplays() {
		player := overlay.players[r.Name]
		player.height.Update(overlay.normalTime)
		player.fade.Update(overlay.normalTime)
		player.fadeHit.Update(overlay.normalTime)
		player.scaleHit.Update(overlay.normalTime)
		player.index.Update(overlay.normalTime)
		player.scoreDisp.Update(overlay.normalTime)
		player.ppDisp.Update(overlay.normalTime)
		player.accDisp.Update(overlay.normalTime)
		player.lastCombo = r.Combo
		if (settings.Knockout.Mode == settings.MaxCombo || settings.Knockout.Mode == settings.ComboBreak) && r.EliminationTime >= 0 && time >= float64(r.EliminationTime) && r.EliminationCombo > 0 && !player.hasBroken {
			graceElapsed := settings.Knockout.Mode == settings.MaxCombo || time > settings.Knockout.GraceEndTime*1000
			if graceElapsed && replayLeaderProtected(r, overlay.controller.GetReplays()) {
				player.protectedBreak = true
			}
			if graceElapsed && !replayLeaderProtected(r, overlay.controller.GetReplays()) && overlay.alivePlayers > settings.Knockout.MinPlayers {
				player.hasBroken = true
				player.breakTime = int64(time)
				overlay.alivePlayers--
				player.fade.AddEvent(overlay.normalTime, overlay.normalTime+500, 0)
				player.height.SetEasing(easing.OutQuad)
				player.height.AddEvent(overlay.normalTime+500, overlay.normalTime+850, 0)
				log.Printf("%s eliminated at pre-simulated max-combo miss: %.0f", r.RawName, time)
			}
		}

		currentHp := overlay.controller.GetRuleset().GetHP(overlay.controller.GetCursors()[player.oldIndex])

		if player.displayHp < currentHp {
			player.displayHp = min(1.0, player.displayHp+math.Abs(currentHp-player.displayHp)/4*delta/16.667)
		} else if player.displayHp > currentHp {
			player.displayHp = max(0.0, player.displayHp-math.Abs(player.displayHp-currentHp)/6*delta/16.667)
		}
	}
}

func (overlay *KnockoutOverlay) SetMusic(music bass.ITrack) {
	overlay.music = music
}

func (player *knockoutPlayer) leaderboardOpacity() float64 {
	if player.protectedBreak {
		return 0.5
	}
	return 1
}

func isSliderTailResult(result osu.JudgementResult) bool {
	return result.MaxResult&(osu.SliderEnd|osu.LegacySliderEnd) != 0
}

func shouldShowKnockoutMiss(result osu.HitResult, eliminated bool) bool {
	return !eliminated && result&osu.BaseHitsM == osu.Miss
}

func knockoutRowScale(totalHeight, baseRowHeight, screenHeight float64) float64 {
	if totalHeight <= 0 || baseRowHeight <= 0 {
		return 1
	}
	return min(48/baseRowHeight, screenHeight*0.94/totalHeight)
}

func (overlay *KnockoutOverlay) visiblePlayers() []*knockoutPlayer {
	players := make([]*knockoutPlayer, 0, len(overlay.playersArray))
	for _, player := range overlay.playersArray {
		if !player.hasBroken || (player.height != nil && player.height.GetValue() > 0.01) {
			players = append(players, player)
		}
	}
	return players
}

func (overlay *KnockoutOverlay) DrawBackground(_ *batch.QuadBatch, _ []color2.Color, _ float64) {}

func (overlay *KnockoutOverlay) DrawBeforeObjects(_ *batch.QuadBatch, _ []color2.Color, _ float64) {}

func (overlay *KnockoutOverlay) DrawNormal(batch *batch.QuadBatch, colors []color2.Color, alpha float64) {
	alpha *= overlay.fade.GetValue()

	scl := 384.0 * (1080.0 / 900.0 * 0.9) / (51)

	alive := 0
	for _, r := range overlay.controller.GetReplays() {
		player := overlay.players[r.Name]
		if !player.hasBroken {
			alive++
		}
	}

	for i := 0; i < len(overlay.deathBubbles); i++ {
		batch.ResetTransform()
		bubble := overlay.deathBubbles[i]
		bubble.deathFade.Update(overlay.normalTime)
		bubble.deathSlide.Update(overlay.normalTime)
		bubble.deathScale.Update(overlay.normalTime)

		if bubble.deathFade.GetValue() >= 0.01 {
			rep := overlay.players[bubble.name]
			name := overlay.controller.GetReplays()[rep.oldIndex].RawName
			size := scl * bubble.deathScale.GetValue()
			width := overlay.font.GetWidth(size, name)
			c := colors[rep.oldIndex]
			overlay.drawPlayerName(batch, bubble.deathX-width/2, bubble.deathSlide.GetValue(), size, false, name, c, alpha*bubble.deathFade.GetValue())

			miss := skin.GetTexture("hit0")
			iconWidth := size * float64(miss.Width) / float64(miss.Height)
			batch.SetColor(1, 1, 1, alpha*bubble.deathFade.GetValue())
			batch.SetSubScale(iconWidth/2, size/2)
			batch.SetTranslation(vector.NewVec2d(bubble.deathX+width/2+size*0.3+iconWidth/2, bubble.deathSlide.GetValue()))
			batch.DrawUnit(*miss)
		}

		if bubble.endTime <= overlay.normalTime {
			overlay.deathBubbles = append(overlay.deathBubbles[:i], overlay.deathBubbles[i+1:]...)
			i--
		}
	}

	minSize := settings.Knockout.MinCursorSize
	maxSize := settings.Knockout.MaxCursorSize
	settings.Cursor.CursorSize = minSize + (maxSize-minSize)*math.Pow(1-math.Sin(float64(alive)/max(51, float64(settings.PLAYERS))*math.Pi/2), 3)

	batch.SetScale(1, 1)
}

func (overlay *KnockoutOverlay) DrawHUD(batch *batch.QuadBatch, colors []color2.Color, alpha float64) {
	alpha *= overlay.fade.GetValue()
	if overlay.intro != nil && settings.START <= 0 && overlay.audioTime < overlay.intro.end {
		alpha = 0 // Opening card takes priority over the player list.
	}

	batch.ResetTransform()

	controller := overlay.controller
	replays := controller.GetReplays()
	visible := overlay.visiblePlayers()

	scl := overlay.ScaledHeight * 0.9 * 1.3 / float64(max(51, settings.PLAYERS+1))
	//margin := scl*0.02

	highestCombo := int64(0)
	highestPP := 0.0
	highestACC := 0.0
	highestScore := int64(0)
	cumulativeHeight := 0.0
	maxPlayerWidth := 0.0
	ppRanks := make(map[string]int, len(replays))
	urRanks := make(map[string]int, len(replays))

	if settings.Knockout.ShowPPAndURRank {
		ranked := append([]dance.RpData(nil), replays...)
		sort.SliceStable(ranked, func(i, j int) bool {
			return overlay.players[ranked[i].Name].pp > overlay.players[ranked[j].Name].pp
		})
		for i, replay := range ranked {
			ppRanks[replay.Name] = i + 1
		}
		if settings.Knockout.ShowRealtimeUR {
			sort.SliceStable(ranked, func(i, j int) bool {
				return overlay.players[ranked[i].Name].ur < overlay.players[ranked[j].Name].ur
			})
			for i, replay := range ranked {
				urRanks[replay.Name] = i + 1
			}
		}
	}

	for _, rep := range visible {
		r := replays[rep.oldIndex]
		cumulativeHeight += rep.height.GetValue()

		highestCombo = max(highestCombo, overlay.players[r.Name].sCombo)
		highestPP = max(highestPP, overlay.players[r.Name].pp)
		highestACC = max(highestACC, r.Accuracy*100)
		highestScore = max(highestScore, overlay.players[r.Name].score)

		pWidth := overlay.font.GetWidth(scl, r.Name)

		if r.Mods != "" {
			pWidth += overlay.font.GetWidth(scl*0.8, "+"+r.Mods)
		}

		maxPlayerWidth = max(maxPlayerWidth, pWidth)
	}

	// Expand the remaining rows into the available height, capped at a readable
	// 48px pitch. Collapsing rows retain their proportional share while fading.
	baseRowHeight := overlay.ScaledHeight * 0.9 * 1.2 / float64(max(51, settings.PLAYERS+1))
	rowScale := knockoutRowScale(cumulativeHeight, baseRowHeight, overlay.ScaledHeight)
	cumulativeHeight *= rowScale

	//cL := strconv.FormatInt(highestCombo, 10)
	cP := strconv.FormatInt(int64(highestPP), 10)
	cA := strconv.FormatInt(int64(highestACC), 10)
	cS := overlay.font.GetWidthMonospaced(scl, utils.Humanize(highestScore))

	accuracy1 := cA + ".00% " + cP + ".00pp"
	if settings.Knockout.ShowRealtimeUR {
		accuracy1 += " 999.99UR"
	}
	if settings.Knockout.ShowPPAndURRank {
		accuracy1 += fmt.Sprintf(" PP#%d", len(replays))
		if settings.Knockout.ShowRealtimeUR {
			accuracy1 += fmt.Sprintf(" UR#%d", len(replays))
		}
	}
	nWidth := overlay.font.GetWidthMonospaced(scl, accuracy1)

	maxLength := 3.2*scl + nWidth + maxPlayerWidth

	xSlideLeft := (overlay.fade.GetValue() - 1.0) * maxLength
	xSlideRight := (1.0 - overlay.fade.GetValue()) * (cS + overlay.font.GetWidthMonospaced(scl, fmt.Sprintf("%dx ", highestCombo)) + 0.5*scl)

	rowPosY := max((overlay.ScaledHeight-cumulativeHeight)/2, scl)
	rowPositions := make(map[*knockoutPlayer]float64, len(visible))
	targetY := rowPosY
	for _, rep := range visible {
		rep.rowY.SetValue(targetY+rep.height.GetValue()*rowScale/2, overlay.audioTime < 0)
		rep.rowY.Update(overlay.normalTime)
		rowPositions[rep] = rep.rowY.GetValue()
		targetY += rep.height.GetValue() * rowScale
	}
	// Draw textures like keys, grade, hit values
	for _, rep := range visible {
		r := replays[rep.oldIndex]
		player := overlay.players[r.Name]

		rowBaseY := rowPositions[rep]

		//batch.SetColor(0.1, 0.8, 0.4, alpha*player.fade.GetValue()*player.leaderboardOpacity()*0.4)
		//add := 0.3 + float64(int(math.Round(rep.index.GetValue()))%2)*0.2
		//batch.SetColor(add, add, add, alpha*player.fade.GetValue()*player.leaderboardOpacity()*0.7)
		//batch.SetAdditive(true)
		//batch.SetSubScale(player.displayHp*30.5*scl*0.9/2, scl*0.9/2)
		//batch.SetTranslation(vector.NewVec2d(player.displayHp*30.5/2*scl*0.9/2 /*rowPosY*/, rowBaseY))
		//batch.DrawUnit(graphics.Pixel.GetRegion())
		//batch.SetSubScale(16.5*scl*0.9/2, scl*0.9/2)
		//batch.SetTranslation(vector.NewVec2d(settings.Graphics.GetWidthF()-16.5/2*scl*0.9/2 /*rowPosY*/, rowBaseY))
		//batch.DrawUnit(graphics.Pixel.GetRegion())
		//batch.SetAdditive(false)

		batch.SetColor(float64(colors[rep.oldIndex].R), float64(colors[rep.oldIndex].G), float64(colors[rep.oldIndex].B), alpha*player.fade.GetValue()*player.leaderboardOpacity())

		for j := range 2 {
			batch.SetSubScale(scl*0.8/2, scl*0.8/2)
			batch.SetTranslation(vector.NewVec2d((float64(j)+0.5)*scl+xSlideLeft, rowBaseY))

			if controller.GetClick(rep.oldIndex, j) || controller.GetClick(rep.oldIndex, j+2) {
				batch.DrawUnit(*overlay.ButtonClicked)
			} else {
				batch.DrawUnit(*overlay.Button)
			}
		}

		width := overlay.font.GetWidth(scl, r.Name)

		batch.SetColor(1, 1, 1, alpha*player.fade.GetValue()*player.leaderboardOpacity())

		if r.Mods != "" {
			width += overlay.font.GetWidth(scl*0.8, "+"+r.Mods)
		}

		if r.Grade != osu.NONE {
			text := skin.GetTexture("ranking-" + r.Grade.TextureName() + "-small")

			ratio := 1.0 / 44.0 // default skin's grade height
			if text.Height < 44 {
				ratio = 1.0 / float64(text.Height) // if skin's grade is smaller, make it bigger
			}

			batch.SetSubScale(scl*0.9*ratio, scl*0.9*ratio)
			batch.SetTranslation(vector.NewVec2d(2.6*scl+nWidth+xSlideLeft, rowBaseY))

			batch.DrawTexture(*text)
		}

		batch.SetColor(1, 1, 1, alpha*player.fade.GetValue()*player.leaderboardOpacity()*player.fadeHit.GetValue())
		//batch.SetSubScale(scl*0.9/2*player.scaleHit.GetValue(), scl*0.9/2*player.scaleHit.GetValue())
		//batch.SetTranslation(vector.NewVec2d(3*scl+width+nWidth+scl*0.5, rowBaseY))

		if player.lastHit != 0 {
			tex := ""

			switch player.lastHit & osu.BaseHitsM {
			case osu.Hit300:
				tex = "hit300"
			case osu.Hit100:
				tex = "hit100"
			case osu.Hit50:
				tex = "hit50"
			case osu.Miss:
				tex = "hit0"
			}

			switch player.lastHit & osu.Additions {
			case osu.KatuAddition:
				tex += "k"
			case osu.GekiAddition:
				tex += "g"
			}

			if tex != "" {
				hitTexture := skin.GetTexture(tex)
				batch.SetSubScale(scl*0.8/2*player.scaleHit.GetValue()*(float64(hitTexture.Width)/float64(hitTexture.Height)), scl*0.8/2*player.scaleHit.GetValue())
				batch.SetTranslation(vector.NewVec2d(3.2*scl+width+nWidth+scl*(float64(hitTexture.Width)/float64(hitTexture.Height))*0.5+xSlideLeft, rowBaseY))
				batch.DrawUnit(*hitTexture)
			}
		}
	}

	batch.ResetTransform()

	rowPosY = max((overlay.ScaledHeight-cumulativeHeight)/2, scl)
	ascScl := overlay.font.GetAscent() * (scl / overlay.font.GetSize()) / 2

	// Draw texts
	for _, rep := range visible {
		r := replays[rep.oldIndex]
		player := overlay.players[r.Name]

		rowBaseY := rowPositions[rep]

		batch.SetColor(1, 1, 1, alpha*player.fade.GetValue()*player.leaderboardOpacity())

		accuracy := fmt.Sprintf("%"+strconv.Itoa(len(cA)+3)+".2f%% %"+strconv.Itoa(len(cP)+3)+".2fpp", overlay.players[r.Name].accDisp.GetValue(), overlay.players[r.Name].ppDisp.GetValue())
		if settings.Knockout.ShowRealtimeUR {
			accuracy += fmt.Sprintf(" %.2fUR", player.ur)
		}
		overlay.font.DrawOrigin(batch, 2*scl+xSlideLeft, rowBaseY, vector.CentreLeft, scl, true, accuracy)
		if settings.Knockout.ShowPPAndURRank {
			rankText := fmt.Sprintf(" PP#%d", ppRanks[r.Name])
			if settings.Knockout.ShowRealtimeUR {
				rankText += fmt.Sprintf(" UR#%d", urRanks[r.Name])
			}
			rankColor := 1.0
			bold := false
			if settings.Knockout.Rank1Highlight && (ppRanks[r.Name] == 1 || (settings.Knockout.ShowRealtimeUR && urRanks[r.Name] == 1)) {
				rankColor = 0.95
				bold = true
			}
			batch.SetColor(1, rankColor, 0.3, alpha*player.fade.GetValue()*player.leaderboardOpacity())
			overlay.font.DrawOrigin(batch, 2*scl+xSlideLeft+overlay.font.GetWidth(scl, accuracy)+scl*0.2, rowBaseY, vector.CentreLeft, scl, bold, rankText)
			batch.SetColor(1, 1, 1, alpha*player.fade.GetValue()*player.leaderboardOpacity())
		}

		scorestr := utils.Humanize(int64(player.scoreDisp.GetValue()))

		sWC := fmt.Sprintf("%dx ", overlay.players[r.Name].sCombo)

		if settings.Knockout.ShowScoreAndCombo {
			overlay.font.DrawOrigin(batch, overlay.ScaledWidth-cS-0.5*scl+xSlideRight, rowBaseY, vector.CentreRight, scl, true, sWC)
			overlay.font.DrawOrigin(batch, overlay.ScaledWidth-0.5*scl+xSlideRight, rowBaseY, vector.CentreRight, scl, true, scorestr)
		}

		overlay.drawPlayerName(batch, 3.2*scl+nWidth+xSlideLeft, rowBaseY, scl, false, r.Name, colors[rep.oldIndex], alpha*player.fade.GetValue()*player.leaderboardOpacity())
		width := overlay.font.GetWidth(scl, r.Name)

		batch.SetColor(1, 1, 1, alpha*player.fade.GetValue()*player.leaderboardOpacity())

		if r.Mods != "" {
			overlay.font.DrawOrigin(batch, 3.2*scl+width+nWidth+xSlideLeft, rowBaseY+ascScl, vector.BottomLeft, scl*0.8, false, "+"+r.Mods)
		}
		if player.protectedBreak {
			modWidth := 0.0
			if r.Mods != "" {
				modWidth = overlay.font.GetWidth(scl*0.8, "+"+r.Mods)
			}
			batch.SetColor(1, 0.75, 0.3, alpha*player.fade.GetValue())
			overlay.font.DrawOrigin(batch, 3.2*scl+width+modWidth+nWidth+xSlideLeft+(3.8*2.0/3.0)*scl, rowBaseY, vector.CentreLeft, scl, false, " [保留]")
		}
	}

	if overlay.resultsReady && overlay.normalTime >= overlay.resultsStart {
		resultsAlpha := min(1, (overlay.normalTime-overlay.resultsStart)/300)
		overlay.drawFinalStandings(batch, colors, resultsAlpha)
	}
	overlay.drawBeatmapIntro(batch)
	if text := settings.Knockout.Watermark; text != "" {
		batch.ResetTransform()
		x, y := overlay.ScaledWidth-28, overlay.ScaledHeight-24
		// A thin shadow keeps translucent white readable without a background box.
		batch.SetColor(0, 0, 0, 0.45)
		overlay.font.DrawOrigin(batch, x+1, y+1, vector.BottomRight, 22, false, text)
		batch.SetColor(1, 1, 1, 0.65)
		overlay.font.DrawOrigin(batch, x, y, vector.BottomRight, 22, false, text)
		batch.ResetTransform()
		batch.SetColor(1, 1, 1, 1)
	}
}

func (overlay *KnockoutOverlay) drawFinalStandings(batch *batch.QuadBatch, colors []color2.Color, alpha float64) {
	pages := standingPages(overlay.finalStandings)
	if len(pages) == 0 {
		return
	}
	pageIndex := min(max(0, int((overlay.normalTime-overlay.resultsStart)/6000)), len(pages)-1)
	panelHeight := min(overlay.ScaledHeight*0.88, 800.0)
	panelWidth := min(overlay.ScaledWidth*0.9, 1500.0)
	panelX, panelY := (overlay.ScaledWidth-panelWidth)/2, (overlay.ScaledHeight-panelHeight)/2
	pixel := graphics.Pixel.GetRegion()
	drawRect := func(x, y, w, h, r, g, b, a float64) {
		batch.ResetTransform()
		batch.SetColor(r, g, b, a*alpha)
		batch.SetSubScale(w/2, h/2)
		batch.SetTranslation(vector.NewVec2d(x+w/2, y+h/2))
		batch.DrawUnit(pixel)
		batch.ResetTransform()
	}
	drawRect(0, 0, overlay.ScaledWidth, overlay.ScaledHeight, 0.01, 0.02, 0.04, 0.72)
	drawRect(panelX, panelY, panelWidth, panelHeight, 0.055, 0.07, 0.10, 0.96)
	drawRect(panelX, panelY, panelWidth, 3, 0.92, 0.72, 0.30, 0.95)
	batch.SetColor(1, 0.84, 0.48, alpha)
	overlay.font.DrawOrigin(batch, overlay.ScaledWidth/2, panelY+32, vector.Centre, 27, true, "荣誉榜")
	batch.SetColor(0.80, 0.85, 0.92, alpha)
	overlay.font.DrawOrigin(batch, panelX+panelWidth-28, panelY+32, vector.CentreRight, 18, false, fmt.Sprintf("%d / %d", pageIndex+1, len(pages)))
	y := panelY + 85
	page := pages[pageIndex]
	if page[0].group != "" {
		batch.SetColor(1, 0.78, 0.4, alpha)
		overlay.font.DrawOrigin(batch, panelX+30, y, vector.CentreLeft, 24, true, page[0].group)
		y += 42
	}
	drawName := func(row finalStanding, x, y, size, width float64) {
		c := color2.NewRGBA(1, 1, 1, 1)
		if row.colorIndex >= 0 && row.colorIndex < len(colors) {
			c = colors[row.colorIndex]
		}
		if measured := overlay.font.GetWidth(size, row.name); measured > width {
			size *= width / measured
		}
		overlay.drawPlayerName(batch, x, y, size, true, row.name, c, alpha)
	}
	if page[0].featured {
		nameX, awardsX := panelX+28, panelX+panelWidth*0.30
		batch.SetColor(0.70, 0.76, 0.84, alpha)
		overlay.font.DrawOrigin(batch, nameX, y, vector.CentreLeft, 15, true, "玩家")
		overlay.font.DrawOrigin(batch, awardsX, y, vector.CentreLeft, 15, true, "保留原因与最终数据")
		y += 30
		for i, block := range page {
			row := block.players[0]
			if i%2 == 0 {
				drawRect(panelX+12, y-20, panelWidth-24, 52, 0.18, 0.21, 0.26, 0.30)
			}
			drawName(row, nameX, y+6, 21, awardsX-nameX-20)
			text := strings.Join(row.awards, " & ")
			size := 19.0
			if width := overlay.font.GetWidth(size, text); width > panelX+panelWidth-28-awardsX {
				size *= (panelX + panelWidth - 28 - awardsX) / width
			}
			batch.SetColor(0.94, 0.96, 1, alpha)
			overlay.font.DrawOrigin(batch, awardsX, y+6, vector.CentreLeft, size, false, text)
			y += 52
		}
	} else {
		block := page[0]
		batch.SetColor(1, 0.84, 0.48, alpha)
		overlay.font.DrawOrigin(batch, panelX+32, y+8, vector.CentreLeft, 24, true, block.heading)
		y += 48
		for i, row := range block.players {
			width := (panelWidth - 64) / 3
			drawName(row, panelX+32+float64(i%3)*width, y+float64(i/3)*46, 23, width-22)
		}
	}

	batch.ResetTransform()
	batch.SetColor(1, 1, 1, 1)
}

func (overlay *KnockoutOverlay) IsBroken(cursor *graphics.Cursor) bool {
	return overlay.players[overlay.names[cursor]].hasBroken
}

func (overlay *KnockoutOverlay) updateBreaks(time float64) {
	inBreak := false

	for _, b := range overlay.controller.GetRuleset().GetBeatMap().Pauses {
		if overlay.audioTime < b.GetStartTime() {
			break
		}

		if b.GetEndTime()-b.GetStartTime() >= 1000 && overlay.audioTime >= b.GetStartTime() && overlay.audioTime <= b.GetEndTime() {
			inBreak = true

			break
		}
	}

	if !overlay.breakMode && inBreak {
		if settings.Knockout.HideOverlayOnBreaks {
			overlay.fade.AddEventEase(time, time+500, 0, easing.OutQuad)
		}
	} else if overlay.breakMode && !inBreak {
		overlay.fade.AddEventEase(time, time+500, 1, easing.OutQuad)
	}

	overlay.breakMode = inBreak
}

func (overlay *KnockoutOverlay) DisableAudioSubmission(_ bool) {}

func (overlay *KnockoutOverlay) ShouldDrawHUDBeforeCursor() bool {
	return false
}

// Player-colored name without an outline.
func (overlay *KnockoutOverlay) drawPlayerName(batch *batch.QuadBatch, x, y, size float64, monospaced bool, name string, c color2.Color, alpha float64) {
	batch.SetColor(float64(c.R), float64(c.G), float64(c.B), alpha)
	overlay.font.DrawOrigin(batch, x, y, vector.CentreLeft, size, monospaced, name)
}
