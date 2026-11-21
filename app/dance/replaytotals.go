package dance

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/rplpa"
)

// Keys include the score namespace: stable IDs and solo-score IDs may overlap.
func replayScoreKey(replay *rplpa.Replay) string {
	if replay.OsuVersion < 30000000 {
		return fmt.Sprintf("legacy:%d", replay.ScoreID)
	}
	id := replay.ScoreID
	if replay.ScoreInfo != nil && replay.ScoreInfo.ScoreId != 0 {
		id = replay.ScoreInfo.ScoreId
	}
	return fmt.Sprintf("solo:%d", id)
}

func loadReplayTotalScores(path string) (map[string]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	columns := map[string]int{}
	for i, name := range header {
		columns[strings.TrimPrefix(strings.TrimSpace(name), "\ufeff")] = i
	}
	for _, name := range []string{"score_id", "legacy_score_id", "total_score"} {
		if _, ok := columns[name]; !ok {
			return nil, fmt.Errorf("replay score CSV missing column %q", name)
		}
	}
	totals := map[string]int64{}
	for line := 2; ; line++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		total, err := strconv.ParseInt(strings.TrimSpace(row[columns["total_score"]]), 10, 64)
		if err != nil || total < 0 {
			return nil, fmt.Errorf("invalid total_score on line %d", line)
		}
		for _, ns := range []struct{ column, prefix string }{{"score_id", "solo:"}, {"legacy_score_id", "legacy:"}} {
			raw := strings.TrimSpace(row[columns[ns.column]])
			if raw == "" {
				continue
			}
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("invalid %s on line %d", ns.column, line)
			}
			key := ns.prefix + strconv.FormatInt(id, 10)
			if old, exists := totals[key]; exists && old != total {
				return nil, fmt.Errorf("conflicting totals for %s", key)
			}
			totals[key] = total
		}
	}
	return totals, nil
}

func replayMods(replay *rplpa.Replay) difficulty.Modifier {
	mods := difficulty.Modifier(replay.Mods)
	if replay.ScoreInfo != nil && len(replay.ScoreInfo.Mods) > 0 {
		d := difficulty.NewDifficulty(5, 5, 5, 5)
		list := make([]rplpa.ModInfo, 0, len(replay.ScoreInfo.Mods))
		for _, m := range replay.ScoreInfo.Mods {
			if m.Acronym == "RD" {
				mods |= difficulty.Random
			}
			list = append(list, *m)
		}
		d.SetMods2(list)
		mods |= d.Mods
	}
	return mods
}
func easyHalfTime(mods difficulty.Modifier) bool {
	return mods&(difficulty.Easy|difficulty.HalfTime|difficulty.Daycore) != 0
}

func bestReplaysPerPlayer(candidates []*rplpa.Replay, totals map[string]int64) ([]*rplpa.Replay, error) {
	best := map[string]*rplpa.Replay{}
	for _, replay := range candidates {
		if settings.Knockout.SplitEasyHalfTime && replayMods(replay).Active(difficulty.Random) {
			continue
		}
		score, ok := totals[replayScoreKey(replay)]
		if !ok {
			return nil, fmt.Errorf("missing lazer total_score for %s (%s)", replay.Username, replayScoreKey(replay))
		}
		name := strings.ToLower(strings.TrimSpace(replay.Username))
		previous := best[name]
		preferNormal := previous != nil && settings.Knockout.SplitEasyHalfTime && easyHalfTime(replayMods(previous)) && !easyHalfTime(replayMods(replay))
		sameGroup := previous == nil || !settings.Knockout.SplitEasyHalfTime || easyHalfTime(replayMods(previous)) == easyHalfTime(replayMods(replay))
		if previous == nil || preferNormal || (sameGroup && score > totals[replayScoreKey(previous)]) {
			best[name] = replay
		}
	}
	result := make([]*rplpa.Replay, 0, len(best))
	for _, replay := range candidates {
		name := strings.ToLower(strings.TrimSpace(replay.Username))
		if best[name] == replay {
			result = append(result, replay)
			delete(best, name)
		}
	}
	return result, nil
}
