package dance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wieku/danser-go/app/rulesets/osu"
)

func TestLoadReplayErrorFixes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixes.json")
	data := `[{"ReplayIndex":1,"ObjectIndex":7,"Result":2,"IsBreak":true,"MaxComboOffset":-1,"NowComboOffset":-1}]`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	fixes, err := loadReplayErrorFixes(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 || fixes[0].ReplayIndex != 1 || fixes[0].ObjectIndex != 7 || fixes[0].Result != 2 || !fixes[0].IsBreak {
		t.Fatalf("unexpected corrections: %#v", fixes)
	}

	if err := os.WriteFile(path, []byte(`[{"ReplayIndex":0,"ObjectIndex":1,"Result":0}]`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadReplayErrorFixes(path); err == nil {
		t.Fatal("invalid replay index was accepted")
	}
}

func TestReplayStatsMatch(t *testing.T) {
	replay := RpData{
		Expected300:      100,
		Expected100:      4,
		Expected50:       1,
		ExpectedMiss:     2,
		ExpectedMaxCombo: 73,
	}
	score := osu.Score{Count300: 100, Count100: 4, Count50: 1, CountMiss: 2, Combo: 73}

	if !replayStatsMatch(replay, score) {
		t.Fatal("matching replay totals were reported as a mismatch")
	}

	score.CountMiss++
	if replayStatsMatch(replay, score) {
		t.Fatal("different replay totals were reported as a match")
	}
}
