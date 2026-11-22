package main

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
)

func (a *app) perPage() int {
	if a.pageSize == 0 {
		return 50
	}
	return a.pageSize
}
func catalogMatch(p leetcode.Problem, q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	return q == "" || strings.Contains(strings.ToLower(p.Title), q) || strings.Contains(strings.ToLower(p.EnglishTitle), q) || strings.Contains(p.FrontendID, q)
}
func (a *app) filterCatalog(q string, skip int) {
	a.matches = nil
	ranks := map[string]int{"Easy": 1, "Medium": 2, "Hard": 3}
	for _, p := range a.catalog {
		if a.statusFilter == "Solved" && !a.solved[p.Slug] || a.statusFilter == "Unsolved" && a.solved[p.Slug] {
			continue
		}
		if (a.hidePremium || a.accessFilter == "Free") && p.Paid || a.accessFilter == "Premium" && !p.Paid {
			continue
		}
		if !catalogMatch(p, q) || len(a.difficulties) > 0 && !a.difficulties[p.Difficulty] {
			continue
		}
		if len(a.tags) > 0 && !slices.ContainsFunc(p.Topics, func(t leetcode.Topic) bool { return a.tags[t.Name] }) {
			continue
		}
		a.matches = append(a.matches, p)
	}
	slices.SortStableFunc(a.matches, func(p, r leetcode.Problem) int {
		var n int
		switch a.sortColumn {
		case "Title":
			n = strings.Compare(strings.ToLower(strings.TrimSpace(p.Title)), strings.ToLower(strings.TrimSpace(r.Title)))
		case "Acceptance":
			var x, y float64
			if p.Acceptance != nil {
				x = *p.Acceptance
			}
			if r.Acceptance != nil {
				y = *r.Acceptance
			}
			n = cmp.Compare(x, y)
		case "Difficulty":
			n = cmp.Compare(ranks[p.Difficulty], ranks[r.Difficulty])
		default:
			x, e := strconv.Atoi(p.FrontendID)
			y, f := strconv.Atoi(r.FrontendID)
			if e == nil && f == nil {
				n = cmp.Compare(x, y)
			} else if e == nil {
				n = -1
			} else if f == nil {
				n = 1
			} else {
				n = strings.Compare(p.FrontendID, r.FrontendID)
			}
		}
		if a.descending {
			return -n
		}
		return n
	})
	size := a.perPage()
	skip = min(max(0, skip), max(0, (len(a.matches)-1)/size)*size)
	a.page = leetcode.Page{Total: len(a.matches), Problems: a.matches[skip:min(skip+size, len(a.matches))]}
	a.skip, a.listed = skip, q
	a.catalogScroll = ui.ScrollState{}
	a.selectedRow = -1
	a.listKeyboardNav = false
	a.hoverRowPlus1 = 0
	a.lastCatalogScrollY = 0
}
func toggleFilter(m *map[string]bool, key string) {
	if *m == nil {
		*m = map[string]bool{}
	}
	if (*m)[key] {
		delete(*m, key)
	} else {
		(*m)[key] = true
	}
}
