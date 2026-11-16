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

func TestUpdatePlayerURIgnoresInvalidSpeed(t *testing.T) {
	player := new(knockoutPlayer)
	updatePlayerUR(player, 10, 0)

	if player.hitErrorCount != 0 || player.ur != 0 {
		t.Fatalf("invalid speed changed player UR state: count=%d UR=%v", player.hitErrorCount, player.ur)
	}
}
