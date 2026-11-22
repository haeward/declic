package main

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/render"
)

const (
	hintIconSVG    = `<svg class="sum-ic" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M5.5 11.5h5M6.5 13.5h3M5.5 9.5c0-1.2-2-2.2-2-4.3a4.5 4.5 0 1 1 9 0c0 2.1-2 3.1-2 4.3"/></svg>`
	similarIconSVG = `<svg class="sum-ic" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="4.5" cy="4.5" r="2"/><circle cx="11.5" cy="4.5" r="2"/><circle cx="8" cy="11.5" r="2"/><path d="M6.2 5.5 7.4 9.6M9.8 5.5 8.6 9.6"/></svg>`
)

func (a *app) statementHTML(source string) string {
	page := render.DescriptionWithAppearance("", source, a.appearance)
	var related []struct {
		Title           string `json:"title"`
		TranslatedTitle string `json:"translatedTitle"`
		Slug            string `json:"titleSlug"`
		Difficulty      string `json:"difficulty"`
		Paid            bool   `json:"isPaidOnly"`
	}
	_ = json.Unmarshal([]byte(a.problem.Similar), &related)

	catalogBySlug := map[string]leetcode.Problem{}
	if len(related) > 0 && len(a.catalog) > 0 {
		for _, p := range a.catalog {
			catalogBySlug[p.Slug] = p
		}
	}

	var tail strings.Builder
	if len(a.problem.Hints) > 0 || len(related) > 0 {
		tail.WriteString(`<div class="extra">`)
		for i, hint := range a.problem.Hints {
			fmt.Fprintf(&tail, `<details><summary><span class="sum-left">%sHint %d</span></summary><div class="hint-body">%s</div></details>`, hintIconSVG, i+1, render.SanitizeHTMLFragment(hint))
		}
		if len(related) > 0 {
			fmt.Fprintf(&tail, `<details><summary><span class="sum-left">%sSimilar Questions (%d)</span></summary><div class="sim-list">`, similarIconSVG, len(related))
			for _, item := range related {
				title := item.Title
				if !a.english && a.site == leetcode.CN && item.TranslatedTitle != "" {
					title = item.TranslatedTitle
				}
				id := ""
				paid := item.Paid
				diff := item.Difficulty
				if p, ok := catalogBySlug[item.Slug]; ok {
					id = p.FrontendID
					if title == "" {
						title = p.Title
					}
					if !paid {
						paid = p.Paid
					}
					if diff == "" {
						diff = p.Difficulty
					}
				}
				href := render.LocalProblemPrefix + url.PathEscape(item.Slug)
				fmt.Fprintf(&tail, `<a class="sim-row" href="%s"><span class="sim-left">`, html.EscapeString(href))
				if id != "" {
					fmt.Fprintf(&tail, `<span class="sim-id">%s.</span>`, html.EscapeString(id))
				}
				fmt.Fprintf(&tail, `<span class="sim-title">%s</span>`, html.EscapeString(title))
				if paid {
					label := "Premium"
					if a.site == leetcode.CN {
						label = "会员"
					}
					fmt.Fprintf(&tail, `<span class="pill pill-pm">%s</span>`, label)
				}
				tail.WriteString(`</span>`)
				if diff != "" {
					cls := "pill-med"
					switch strings.ToLower(diff) {
					case "easy":
						cls = "pill-easy"
					case "hard":
						cls = "pill-hard"
					}
					fmt.Fprintf(&tail, `<span class="pill %s">%s</span>`, cls, html.EscapeString(diff))
				}
				tail.WriteString(`</a>`)
			}
			tail.WriteString(`</div></details>`)
		}
		tail.WriteString(`</div>`)
	}
	page = strings.Replace(page, "</style>", fmt.Sprintf("body{font-size:%dpx}</style>", max(13, a.descriptionSize)), 1)
	return strings.Replace(page, "</body>", tail.String()+"</body>", 1)
}
