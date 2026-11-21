package overlays

import (
	"fmt"
	"testing"
)

func TestStandingPagesSeparateGroups(t *testing.T) {
	pages := standingPages([]finalStanding{{name: "a", group: "normal", awards: []string{"a"}}, {name: "b", group: "easy", awards: []string{"a"}}})
	if len(pages) != 2 {
		t.Fatal(pages)
	}
}
func TestStandingThresholdPerAwardAndRepeatedPlayers(t *testing.T) {
	for _, n := range []int{5, 6} {
		rows := []finalStanding{}
		for i := 0; i < n; i++ {
			rows = append(rows, finalStanding{name: fmt.Sprint(i), awards: []string{"a", "b"}})
		}
		pages := standingPages(rows)
		if n == 5 {
			if len(pages) != 1 || !pages[0][0].featured || len(pages[0]) != 5 {
				t.Fatal(pages)
			}
		} else {
			if len(pages) != 2 {
				t.Fatal(pages)
			}
			for _, p := range pages {
				if p[0].featured || len(p[0].players) != 6 {
					t.Fatal(p)
				}
			}
		}
	}
}
func TestStandingContinuationAndClassicFirst(t *testing.T) {
	rows := []finalStanding{{name: "special", awards: []string{"rare", "common"}}}
	for i := 0; i < 40; i++ {
		rows = append(rows, finalStanding{name: fmt.Sprint(i), awards: []string{"common"}})
	}
	pages := standingPages(rows)
	if len(pages) != 3 || !pages[0][0].featured || pages[0][0].players[0].awards[0] != "rare" {
		t.Fatal(pages)
	}
	if pages[1][0].heading != "common" || len(pages[1][0].players) != 36 || pages[2][0].heading != "common (续上页)" || len(pages[2][0].players) != 5 {
		t.Fatal(pages)
	}
}
