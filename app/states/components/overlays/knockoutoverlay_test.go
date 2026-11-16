package overlays

import (
	"math"
	"testing"

	"github.com/wieku/danser-go/app/dance"
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
