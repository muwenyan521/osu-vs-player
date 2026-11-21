package dance

import (
	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/rplpa"
	"os"
	"path/filepath"
	"testing"
)

func TestReplayTotalScoresNamespaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.csv")
	if err := os.WriteFile(path, []byte("replay_file,score_id,legacy_score_id,total_score\nlegacy.osr,123,45,862022\nlazer.osr,45,,869240\n"), 0600); err != nil {
		t.Fatal(err)
	}
	totals, err := loadReplayTotalScores(path)
	if err != nil {
		t.Fatal(err)
	}
	stable := &rplpa.Replay{OsuVersion: 20200101, ScoreID: 45}
	lazer := &rplpa.Replay{OsuVersion: 30000019, ScoreID: 0, ScoreInfo: &rplpa.ScoreInfo{ScoreId: 45}}
	if totals[replayScoreKey(stable)] != 862022 || totals[replayScoreKey(lazer)] != 869240 {
		t.Fatalf("wrong score namespace: %v", totals)
	}
}
func TestReplayTotalScoresRejectConflicts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.csv")
	for _, data := range []string{"score_id,legacy_score_id,total_score\n1,,10\n1,,11\n", "score_id,legacy_score_id,total_score\n1,,-1\n", "score_id,total_score\n1,1\n"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadReplayTotalScores(path); err == nil {
			t.Fatalf("accepted invalid CSV: %s", data)
		}
	}
}

func TestBestReplayUsesStandardizedScoreNotLegacy(t *testing.T) {
	stable := &rplpa.Replay{Username: "mrekk", OsuVersion: 20200101, ScoreID: 1, Score: 99999999}
	lazer := &rplpa.Replay{Username: "MREKK", OsuVersion: 30000019, ScoreID: 2, Score: 869240}
	other := &rplpa.Replay{Username: "other", OsuVersion: 20200101, ScoreID: 3}
	result, err := bestReplaysPerPlayer([]*rplpa.Replay{stable, lazer, other}, map[string]int64{"legacy:1": 734745, "solo:2": 869240, "legacy:3": 100})
	if err != nil || len(result) != 2 || result[0] != lazer || result[1] != other {
		t.Fatalf("wrong selected replays: %v %v", result, err)
	}
}

func TestGroupedSelectionPrefersNormalAndRejectsRandom(t *testing.T) {
	old := settings.Knockout.SplitEasyHalfTime
	settings.Knockout.SplitEasyHalfTime = true
	defer func() { settings.Knockout.SplitEasyHalfTime = old }()
	ht := &rplpa.Replay{Username: "A", OsuVersion: 20200101, ScoreID: 1, Mods: uint32(difficulty.HalfTime)}
	normal := &rplpa.Replay{Username: "A", OsuVersion: 20200101, ScoreID: 2}
	random := &rplpa.Replay{Username: "B", OsuVersion: 20200101, ScoreID: 3, Mods: uint32(difficulty.Random)}
	got, err := bestReplaysPerPlayer([]*rplpa.Replay{ht, normal, random}, map[string]int64{"legacy:1": 999, "legacy:2": 100, "legacy:3": 1000})
	if err != nil || len(got) != 1 || got[0] != normal {
		t.Fatalf("wrong grouped selection: %v %v", got, err)
	}
}

func TestLazerRandomAcronymExcluded(t *testing.T) {
	old := settings.Knockout.SplitEasyHalfTime
	settings.Knockout.SplitEasyHalfTime = true
	defer func() { settings.Knockout.SplitEasyHalfTime = old }()
	random := &rplpa.Replay{Username: "random", OsuVersion: 30000016, ScoreInfo: &rplpa.ScoreInfo{ScoreId: 7, Mods: []*rplpa.ModInfo{{Acronym: "RD"}}}}
	got, err := bestReplaysPerPlayer([]*rplpa.Replay{random}, map[string]int64{})
	if err != nil || len(got) != 0 {
		t.Fatalf("lazer RD was not excluded: %v %v", got, err)
	}
}
