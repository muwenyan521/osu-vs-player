package overlays

import (
	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/math/animation"
	"math"
	"strings"
	"testing"

	"github.com/wieku/danser-go/app/dance"
	"github.com/wieku/danser-go/app/rulesets/osu"
)

func TestBrokeRecordedMaxCombo(t *testing.T) {
	if brokeRecordedMaxCombo(10, 2000) {
		t.Fatal("early combo break counted as a break of the replay max combo")
	}
	if !brokeRecordedMaxCombo(2000, 2000) {
		t.Fatal("break at the replay max combo was not recognized")
	}
	if !brokeRecordedMaxCombo(2001, 2000) {
		t.Fatal("combo beyond the recorded max was not recognized as a break at peak")
	}
}

func TestBuildFinalStandingsDeduplicatesMultiAwardPlayers(t *testing.T) {
	replays := []dance.RpData{
		{RawName: "Alice", Name: "Alice-one", ExpectedScore: 1000, ExpectedPP: 300, ExpectedMaxCombo: 150, ExpectedMiss: 1, HasExpected: true},
		{RawName: "Bob", Name: "Bob", ExpectedScore: 2000, ExpectedPP: 100, ExpectedMaxCombo: 300, ExpectedMiss: 4, HasExpected: true},
		{RawName: "ALICE", Name: "Alice-two", ExpectedScore: 900, ExpectedPP: 120, ExpectedMaxCombo: 100, ExpectedMiss: 2, HasExpected: true},
	}

	standings := buildFinalStandings(replays)
	if len(standings) != 2 {
		t.Fatalf("got %d rows, want 2 deduplicated players: %#v", len(standings), standings)
	}
	if len(standings[0].awards) != 2 || len(standings[1].awards) != 2 {
		t.Fatalf("expected two awards per player, got %#v", standings)
	}
}

func TestReplayLeaderProtectionUsesFinalReplayStats(t *testing.T) {
	replays := []dance.RpData{
		{Name: "score", ExpectedScore: 1000, ExpectedPP: 100, ExpectedMaxCombo: 500, ExpectedMiss: 2, HasExpected: true},
		{Name: "pp", ExpectedScore: 800, ExpectedPP: 300, ExpectedMaxCombo: 400, ExpectedMiss: 3, HasExpected: true},
		{Name: "combo", ExpectedScore: 700, ExpectedPP: 80, ExpectedMaxCombo: 600, ExpectedMiss: 4, HasExpected: true},
		{Name: "miss", ExpectedScore: 650, ExpectedPP: 70, ExpectedMaxCombo: 350, ExpectedMiss: 1, HasExpected: true},
		{Name: "unprotected", ExpectedScore: 400, ExpectedPP: 50, ExpectedMaxCombo: 200, ExpectedMiss: 5, HasExpected: true},
	}

	for _, name := range []string{"score", "pp", "combo", "miss"} {
		if !replayLeaderProtected(replayByName(replays, name), replays) {
			t.Errorf("final-data leader %q was not protected", name)
		}
	}
	if replayLeaderProtected(replayByName(replays, "unprotected"), replays) {
		t.Fatal("non-leader was protected")
	}

	perfect := dance.RpData{Name: "perfect", ExpectedScore: 300, ExpectedPP: 30, ExpectedMaxCombo: 100, ExpectedMiss: 0, HasExpected: true}
	replays = append(replays, perfect)
	if replayLeaderProtected(replayByName(replays, "unprotected"), replays) {
		t.Fatal("fewest-miss protection remained enabled when a replay had zero misses")
	}
}

func replayByName(replays []dance.RpData, name string) dance.RpData {
	for _, replay := range replays {
		if replay.Name == name {
			return replay
		}
	}
	return dance.RpData{}
}

func TestUpdatePlayerUR(t *testing.T) {
	player := new(knockoutPlayer)
	updatePlayerUR(player, -10, 2)
	updatePlayerUR(player, 10, 2)

	if math.Abs(player.ur-50) > 1e-9 {
		t.Fatalf("UR = %v, want 50", player.ur)
	}
}

func TestParseKnockoutPlayerColors(t *testing.T) {
	colors := parseKnockoutPlayerColors("Player:255,128,0|Broken:1,2|Bad:256,0,0")
	color, ok := colors["Player"]
	if !ok {
		t.Fatal("valid player color was not parsed")
	}
	if color.R != 1 || math.Abs(float64(color.G)-128.0/255) > 1e-6 || color.B != 0 || color.A != 1 {
		t.Fatalf("parsed color = %#v", color)
	}
	if _, ok := colors["Broken"]; ok {
		t.Fatal("malformed color was accepted")
	}
	if _, ok := colors["Bad"]; ok {
		t.Fatal("out-of-range color was accepted")
	}
}

func TestUpdatePlayerURIgnoresInvalidSpeed(t *testing.T) {
	player := new(knockoutPlayer)
	updatePlayerUR(player, 10, 0)

	if player.hitErrorCount != 0 || player.ur != 0 {
		t.Fatalf("invalid speed changed player UR state: count=%d UR=%v", player.hitErrorCount, player.ur)
	}
}

func TestFinalStandingsOmitsMissAwardWhenZeroMissReplayExists(t *testing.T) {
	replays := []dance.RpData{
		{RawName: "Leader", Name: "leader", ExpectedScore: 3000, ExpectedPP: 400, ExpectedMaxCombo: 1000, ExpectedMiss: 2, HasExpected: true},
		{RawName: "FC one", Name: "fc1", ExpectedScore: 2000, ExpectedPP: 200, ExpectedMaxCombo: 800, ExpectedMiss: 0, HasExpected: true},
		{RawName: "FC two", Name: "fc2", ExpectedScore: 1000, ExpectedPP: 100, ExpectedMaxCombo: 700, ExpectedMiss: 0, HasExpected: true},
	}
	standings := buildFinalStandings(replays)
	if len(standings) != 1 || len(standings[0].awards) != 3 {
		t.Fatalf("expected only the multi-award leader, got %#v", standings)
	}
	for _, row := range standings {
		for _, award := range row.awards {
			if strings.HasPrefix(award, "最少失误：") {
				t.Fatalf("unexpected miss award: %s", award)
			}
		}
	}
}

func TestVisiblePlayersImmediatelyExcludesEliminatedPlayers(t *testing.T) {
	first := &knockoutPlayer{name: "first"}
	second := &knockoutPlayer{name: "second"}
	third := &knockoutPlayer{name: "third"}
	overlay := &KnockoutOverlay{playersArray: []*knockoutPlayer{first, second, third}}
	second.hasBroken = true
	visible := overlay.visiblePlayers()
	if len(visible) != 2 || visible[0] != first || visible[1] != third {
		t.Fatalf("eliminated player remains in the leaderboard or row order changed: %#v", visible)
	}
}

func TestKnockoutMarkersOnlyShowMissesFromActivePlayers(t *testing.T) {
	for _, result := range []osu.HitResult{osu.Hit300, osu.Hit100, osu.Hit50, osu.SliderMiss, osu.PositionalMiss} {
		if shouldShowKnockoutMiss(result, false) {
			t.Errorf("non-miss result %v produced an in-playfield marker", result)
		}
	}
	if !shouldShowKnockoutMiss(osu.Miss, false) {
		t.Fatal("active player's miss did not produce a marker")
	}
	if shouldShowKnockoutMiss(osu.Miss, true) {
		t.Fatal("already-eliminated player produced a marker")
	}
}

func TestAccuracyLeadersProtectedAndAwarded(t *testing.T) {
	replays := []dance.RpData{
		{RawName: "Score", Name: "score", ExpectedScore: 1000, ExpectedPP: 300, ExpectedMaxCombo: 1000, Expected300: 90, Expected100: 5, ExpectedMiss: 5, HasExpected: true},
		{RawName: "Acc", Name: "acc", ExpectedScore: 900, ExpectedPP: 200, ExpectedMaxCombo: 900, Expected300: 98, Expected100: 1, ExpectedMiss: 1, HasExpected: true},
		{RawName: "Tie", Name: "tie", ExpectedScore: 800, ExpectedPP: 100, ExpectedMaxCombo: 800, Expected300: 98, Expected100: 1, ExpectedMiss: 1, HasExpected: true},
	}
	for _, replay := range replays[1:] {
		if !replayLeaderProtected(replay, replays) {
			t.Fatalf("accuracy leader %s is not protected", replay.Name)
		}
	}
	rows := buildFinalStandings(replays)
	for _, row := range rows[1:] {
		if len(row.awards) != 2 || row.awards[0] != "最高准确率：98.3333%" {
			t.Fatalf("accuracy and miss awards missing: %#v", row)
		}
	}
}

func TestSliderTailResultsAreHidden(t *testing.T) {
	for _, tail := range []osu.HitResult{osu.SliderEnd, osu.LegacySliderEnd} {
		for _, hit := range []osu.HitResult{osu.SliderMiss, osu.Miss, tail} {
			if !isSliderTailResult(osu.JudgementResult{HitResult: hit, MaxResult: tail}) {
				t.Fatalf("slider tail was not hidden: result=%v max=%v", hit, tail)
			}
		}
	}
	for _, max := range []osu.HitResult{osu.Hit300, osu.SliderStart, osu.SliderPoint, osu.SliderRepeat} {
		if isSliderTailResult(osu.JudgementResult{HitResult: osu.Miss, MaxResult: max}) {
			t.Fatalf("non-tail miss incorrectly hidden: max=%v", max)
		}
	}
}

func TestProtectedBreakDimsRowWithoutRemovingPlayer(t *testing.T) {
	player := &knockoutPlayer{name: "protected"}
	if player.leaderboardOpacity() != 1 {
		t.Fatal("active player should keep full row opacity")
	}
	player.protectedBreak = true
	if player.leaderboardOpacity() != 0.5 {
		t.Fatal("protected player should have a dimmed row after breaking")
	}
	overlay := &KnockoutOverlay{playersArray: []*knockoutPlayer{player}}
	if len(overlay.visiblePlayers()) != 1 || player.hasBroken {
		t.Fatal("protected break must not remove the player from the leaderboard")
	}
}

func TestStandardizedScoreLeader(t *testing.T) {
	replays := []dance.RpData{
		{Name: "EZChamp", RawName: "EZChamp", ExpectedScore: 862022, LegacyScore: 116847332, ExpectedPP: 729.21, ExpectedMaxCombo: 2897, ExpectedMiss: 5, HasExpected: true},
		{Name: "mrekk", RawName: "mrekk", ExpectedScore: 869240, LegacyScore: 869240, ExpectedPP: 500, ExpectedMaxCombo: 2728, ExpectedMiss: 20, HasExpected: true},
	}
	if !replayLeaderProtected(replays[1], replays) {
		t.Fatal("standardized score leader is not protected")
	}
	rows := buildFinalStandings(replays)
	for _, row := range rows {
		for _, award := range row.awards {
			if strings.HasPrefix(award, "最高分数：") && (row.name != "mrekk" || award != "最高分数：869,240") {
				t.Fatalf("wrong score award: %#v", row)
			}
		}
	}
}

func TestEliminatedRowRemainsUntilCollapseCompletes(t *testing.T) {
	player := &knockoutPlayer{hasBroken: true, height: animation.NewGlider(20)}
	overlay := &KnockoutOverlay{playersArray: []*knockoutPlayer{player}}
	if len(overlay.visiblePlayers()) != 1 {
		t.Fatal("fading row disappeared immediately")
	}
	player.height.SetValue(0)
	if len(overlay.visiblePlayers()) != 0 {
		t.Fatal("collapsed row is still visible")
	}
}

func TestKnockoutSpacingExpandsAsPlayersLeave(t *testing.T) {
	base := 1080.0 * 0.9 * 1.2 / 69
	previous := 0.0
	for _, count := range []int{68, 40, 20, 10, 3} {
		scale := knockoutRowScale(float64(count)*base, base, 1080)
		pitch := base * scale
		if pitch < previous || pitch > 48.000001 {
			t.Fatalf("invalid pitch for %d players: %f", count, pitch)
		}
		if float64(count)*pitch > 1080*0.94+0.000001 {
			t.Fatal("leaderboard extends beyond available height")
		}
		previous = pitch
	}
	if math.Abs(previous-48) > 0.000001 {
		t.Fatalf("few-player pitch=%f, want 48", previous)
	}
}

func TestSplitAwardsProtectGroupLeaders(t *testing.T) {
	old := settings.Knockout.SplitEasyHalfTime
	settings.Knockout.SplitEasyHalfTime = true
	defer func() { settings.Knockout.SplitEasyHalfTime = old }()
	r := []dance.RpData{{Name: "normal", RawName: "normal", ExpectedScore: 100, ExpectedPP: 100, ExpectedMaxCombo: 10, ExpectedMiss: 1, HasExpected: true}, {Name: "easy", RawName: "easy", ModsV: difficulty.Easy | difficulty.NoFail | difficulty.Lazer, ExpectedScore: 900, ExpectedPP: 900, ExpectedMaxCombo: 100, ExpectedMiss: 2, HasExpected: true}}
	for _, p := range r {
		if !replayLeaderProtected(p, r) {
			t.Fatal("group leader not protected")
		}
	}
	rows := buildFinalStandings(r)
	if len(rows) != 2 || rows[0].group == rows[1].group || !strings.Contains(rows[1].name, "+NFEZ") || strings.Contains(rows[1].name, "LZ") {
		t.Fatalf("invalid grouped standings %#v", rows)
	}
}

func TestProtectionRequiresAnAwardWithAtMostFiveWinners(t *testing.T) {
	old := settings.Knockout.SplitEasyHalfTime
	settings.Knockout.SplitEasyHalfTime = false
	defer func() { settings.Knockout.SplitEasyHalfTime = old }()
	rows := make([]dance.RpData, 6)
	for i := range rows {
		rows[i] = dance.RpData{ExpectedScore: 100, ExpectedPP: 100, ExpectedMaxCombo: 100, Expected300: 99, ExpectedMiss: 1, HasExpected: true}
	}
	if replayLeaderProtected(rows[0], rows) {
		t.Fatal("all awards have six winners")
	}
	if !replayLeaderProtected(rows[0], rows[:5]) {
		t.Fatal("five winners must still be protected")
	}
	rows[0].ExpectedScore = 101
	if !replayLeaderProtected(rows[0], rows) {
		t.Fatal("unique score award must protect despite crowded other awards")
	}
	if replayLeaderProtected(rows[1], rows) {
		t.Fatal("only crowded awards must not protect")
	}
	settings.Knockout.SplitEasyHalfTime = true
	rows[5].ModsV = difficulty.Easy
	if !replayLeaderProtected(rows[1], rows) {
		t.Fatal("normal group has only five tied winners")
	}
}
