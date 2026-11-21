package overlays

// Small award lists use classic player rows; large awards get dedicated pages.
type standingBlock struct {
	heading  string
	group    string
	players  []finalStanding
	featured bool
}

func standingPages(rows []finalStanding) [][]standingBlock {
	pages := [][]standingBlock{}
	groups := []string{}
	for _, r := range rows {
		found := false
		for _, g := range groups {
			if g == r.group {
				found = true
			}
		}
		if !found {
			groups = append(groups, r.group)
		}
	}
	for _, group := range groups {
		awards := []string{}
		winners := map[string][]finalStanding{}
		for _, r := range rows {
			if r.group != group {
				continue
			}
			for _, award := range r.awards {
				if _, exists := winners[award]; !exists {
					awards = append(awards, award)
				}
				winners[award] = append(winners[award], r)
			}
		}
		classic := []standingBlock{}
		for _, r := range rows {
			if r.group != group {
				continue
			}
			remaining := []string{}
			for _, award := range r.awards {
				if len(winners[award]) <= 5 {
					remaining = append(remaining, award)
				}
			}
			if len(remaining) > 0 {
				r.awards = remaining
				classic = append(classic, standingBlock{group: group, players: []finalStanding{r}, featured: true})
			}
		}
		for len(classic) > 0 {
			n := min(10, len(classic))
			pages = append(pages, classic[:n])
			classic = classic[n:]
		}
		for _, award := range awards {
			players := winners[award]
			if len(players) <= 5 {
				continue
			}
			continued := false
			for len(players) > 0 {
				n := min(36, len(players))
				heading := award
				if continued {
					heading += " (续上页)"
				}
				pages = append(pages, []standingBlock{{heading: heading, group: group, players: players[:n]}})
				players = players[n:]
				continued = true
			}
		}
	}
	return pages
}

func (overlay *KnockoutOverlay) RequiredResultsDuration() float64 {
	return float64(max(1, len(standingPages(overlay.finalStandings)))) * 6
}
