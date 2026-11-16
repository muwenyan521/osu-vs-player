package overlays

import (
	"math"
	"testing"
)

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
