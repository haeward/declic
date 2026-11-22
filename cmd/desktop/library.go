package main

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
)

func (a *app) selectedDifficulty() string {
	if len(a.difficulties) != 1 {
		return ""
	}
	for k, v := range a.difficulties {
		if v {
			return k
		}
	}
	return ""
}

func (a *app) selectDifficulty(d string) {
	if d == "" || a.selectedDifficulty() == d {
		a.difficulties = nil
	} else {
		a.difficulties = map[string]bool{d: true}
	}
	a.filterCatalog(a.search, 0)
}

func (a *app) toggleSort(key string) {
	if a.sortColumn != key {
		a.sortColumn = key
		a.descending = false
	} else if !a.descending {
		a.descending = true
	} else {
		a.sortColumn = ""
		a.descending = false
	}
	a.filterCatalog(a.search, 0)
}

func (a *app) problemsView(c *ui.Context) {
	t := c.Theme()
	size := a.perPage()
	totalPages := max(1, (a.page.Total+size-1)/size)

	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Background(t.Surface).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).FillWidth().MaxWidth(1040).MinHeight(0).Padding(22, 32, 0, 32).Gap(16).Children(func() {
			a.dailyView(c)
			hasActiveFilters := a.statusFilter != "" || a.accessFilter != "" || len(a.tags) > 0
			tableGap := float32(14)
			if hasActiveFilters {
				tableGap = 8
			}
			ui.Column(c).Grow(1).FillWidth().MinHeight(0).Gap(tableGap).Children(func() {
				// Single-row toolbar: Search + Difficulty (text) + Sort / Filters / Pick one (icon-only)
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Grow(1).MinWidth(180).Height(38).Padding(0, 12).Gap(9).Radius(10).Border(1, t.Border).Background(t.Surface).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconSearch).FontSize(15).TextColor(t.TextMuted)
						input := ui.TextInput(c.Key("problem-search"), &a.search).Label("Search problems").Placeholder("Search by title or number").Grow(1).MinWidth(80).Border(0, ui.Transparent).Background(ui.Transparent).Padding(0)
						if primaryShortcut(c, ui.KeyF) {
							input.Focus()
						}
						if a.pendingQuery != a.search {
							a.pendingQuery = a.search
							a.queryAt = time.Now()
						}
						if a.search != a.listed && a.catalog != nil && time.Since(a.queryAt) >= 300*time.Millisecond {
							a.filterCatalog(a.search, 0)
						}
						if !hasActiveFilters {
							statusText := ""
							if a.catalogLoading {
								if a.page.Total > 0 {
									statusText = fmt.Sprintf("%d problems · Updating…", a.page.Total)
								} else {
									statusText = "Loading…"
								}
							} else if a.page.Total > 0 {
								statusText = fmt.Sprintf("%d problems", a.page.Total)
							}
							if statusText != "" {
								ui.Text(c, statusText).FontSize(11.5).TextColor(t.TextMuted.Alpha(.75)).SingleLine()
							}
						}
						ui.Text(c, shortcutBadge("F")).FontSize(11).TextColor(t.TextMuted).Background(chipBg(c)).Radius(5).Padding(2, 6)
					})

					activeDiff := a.selectedDifficulty()
					segmentedTrack(c, func() {
						allFg := t.TextMuted
						if activeDiff == "" {
							allFg = t.Text
						}
						segmentItem(c, "All", activeDiff == "", allFg, 32).Disabled(a.catalog == nil).OnClick(func() {
							a.selectDifficulty("")
						})
						for _, difficulty := range []string{"Easy", "Medium", "Hard"} {
							fg, _ := difficultyBadgeColors(c, difficulty)
							segmentItem(c, difficulty, activeDiff == difficulty, fg, 32).Disabled(a.catalog == nil).OnClick(func() {
								a.selectDifficulty(difficulty)
							})
						}
					})

					// Sort icon button + popover
					sortTip := "Sort"
					if a.sortColumn != "" {
						arrow := "↑"
						if a.descending {
							arrow = "↓"
						}
						sortName := a.sortColumn
						if sortName == "#" {
							sortName = "ID"
						}
						sortTip = fmt.Sprintf("Sort: %s %s", sortName, arrow)
					}
					sortAnchor := iconToolbarButton(c, "Sort", sortTip, iconSort, a.sortColumn != "", 0).Disabled(a.catalog == nil).OnClick(func() {
						a.sortOpen = !a.sortOpen
						if a.sortOpen {
							a.filterOpen = false
						}
					})
					ui.PopoverBase(c, sortAnchor, &a.sortOpen, func(panel ui.Element) {
						panel.Width(200).Padding(10).Radius(12).Background(t.Surface).Border(1, t.Border.Alpha(.5)).Shadow(0, 8, 28, 0, ui.RGBA(0, 0, 0, .18))
						ui.Column(c).FillWidth().Gap(2).Children(func() {
							ui.Text(c, "Sort by").FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(2, 10, 6, 10)
							for _, opt := range []struct{ key, label string }{
								{"#", "ID"},
								{"Title", "Title"},
								{"Acceptance", "Acceptance"},
								{"Difficulty", "Difficulty"},
							} {
								active := a.sortColumn == opt.key
								b := ui.ButtonBase(c).Label(opt.label).FillWidth().Height(32).Padding(0, 10).Radius(8)
								if active {
									b.Background(activeChipBg(c))
								} else if b.Hovered() {
									b.Background(t.SurfaceHover)
								}
								b.Children(func() {
									ui.Row(c).Fill().AlignItems(ui.Center).Children(func() {
										fg := t.Text
										weight := 400
										if active {
											fg = accentLinkColor(c)
											weight = 600
										}
										ui.Text(c, opt.label).FontSize(13).FontWeight(weight).TextColor(fg).Grow(1)
										if active {
											dir := "↑"
											if a.descending {
												dir = "↓"
											}
											ui.Text(c, dir).FontSize(12).FontWeight(600).TextColor(accentLinkColor(c))
										}
									})
								}).OnClick(func() { a.toggleSort(opt.key) })
							}
							if a.sortColumn != "" {
								ui.Box(c).FillWidth().Height(1).Background(hairlineColor(c)).Margin(4, 0)
								popoverMenuRow(c, "Reset order").OnClick(func() {
									a.sortColumn = ""
									a.descending = false
									a.sortOpen = false
									a.filterCatalog(a.search, 0)
								})
							}
						})
					})

					// Filters icon button + popover
					count := len(a.tags)
					if a.statusFilter != "" {
						count++
					}
					if a.accessFilter != "" {
						count++
					}
					filterTip := "Filters"
					if count > 0 {
						filterTip = fmt.Sprintf("Filters (%d active)", count)
					}
					filterAnchor := iconToolbarButton(c, "Filters", filterTip, iconFilter, count > 0, count).Disabled(a.catalog == nil).OnClick(func() {
						a.filterOpen = !a.filterOpen
						if a.filterOpen {
							a.sortOpen = false
						}
					})
					ui.PopoverBase(c, filterAnchor, &a.filterOpen, func(panel ui.Element) {
						panel.Width(268).Padding(14).Radius(12).Background(t.Surface).Border(1, t.Border.Alpha(.5)).Shadow(0, 8, 28, 0, ui.RGBA(0, 0, 0, .18))
						ui.Column(c).FillWidth().Gap(10).Children(func() {
							ui.Text(c, "Status").FontSize(12).FontWeight(600).TextColor(t.TextMuted)
							segmentedTrack(c, func() {
								for _, label := range []string{"All", "Unsolved", "Solved"} {
									v := label
									if label == "All" {
										v = ""
									}
									fg := t.TextMuted
									if a.statusFilter == v {
										fg = t.Text
									}
									segmentItem(c, label, a.statusFilter == v, fg, 26).Grow(1).Center().Disabled(v != "" && a.solved == nil).OnClick(func() { a.statusFilter = v; a.filterCatalog(a.search, 0) })
								}
							}).FillWidth()
							if a.username == "" {
								ui.Text(c, "Sign in to filter by solve status.").FontSize(11).TextColor(t.TextMuted)
							}
							if a.solvedNotice != "" {
								ui.Text(c, a.solvedNotice).FontSize(11).TextColor(t.TextMuted)
								linkButton(c, "Retry status").OnClick(a.refreshSolved)
							}
							ui.Text(c, "Access").FontSize(12).FontWeight(600).TextColor(t.TextMuted)
							segmentedTrack(c, func() {
								for _, label := range []string{"All", "Free", "Premium"} {
									v := label
									if label == "All" {
										v = ""
									}
									fg := t.TextMuted
									if a.accessFilter == v {
										fg = t.Text
									}
									segmentItem(c, label, a.accessFilter == v, fg, 26).Grow(1).Center().OnClick(func() { a.accessFilter = v; a.filterCatalog(a.search, 0) })
								}
							}).FillWidth()
							ui.Text(c, "Topics").FontSize(12).FontWeight(600).TextColor(t.TextMuted)
							ui.TextInput(c, &a.tagSearch).Placeholder("Search topics").FillWidth()
							counts := map[string]int{}
							for _, p := range a.catalog {
								for _, topic := range p.Topics {
									counts[topic.Name]++
								}
							}
							ui.Scroll(c).Scrollbars(false).FillWidth().Height(196).Children(func() {
								ui.Column(c).FillWidth().Gap(2).Children(func() {
									for _, topic := range slices.Sorted(maps.Keys(counts)) {
										if strings.Contains(strings.ToLower(topic), strings.ToLower(a.tagSearch)) {
											row := popoverMenuRow(c, fmt.Sprintf("%s · %d", topic, counts[topic]))
											if a.tags[topic] {
												row.Background(activeChipBg(c))
											}
											row.OnClick(func() { toggleFilter(&a.tags, topic); a.filterCatalog(a.search, 0) })
										}
									}
								})
							})
							ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
								linkButton(c, "Reset").OnClick(func() {
									a.tags = nil
									a.statusFilter = ""
									a.accessFilter = ""
									a.filterCatalog(a.search, 0)
								})
							})
						})
					})

					// Pick one (Random) icon button
					iconToolbarButton(c, "Pick one", "Pick one", iconShuffle, false, 0).Disabled(a.busy || !slices.ContainsFunc(a.matches, func(p leetcode.Problem) bool { return !p.Paid || a.premium })).OnClick(func() {
						var slug string
						n := 0
						for _, p := range a.matches {
							if !p.Paid || a.premium {
								n++
								if rand.IntN(n) == 0 {
									slug = p.Slug
								}
							}
						}
						if slug != "" {
							a.open(slug)
						}
					})
				})

				// Active filter chips + problem count (only rendered when filters are active)
				if hasActiveFilters {
					ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
						if a.statusFilter != "" {
							chipButton(c, a.statusFilter+" ×", true).OnClick(func() { a.statusFilter = ""; a.filterCatalog(a.search, 0) })
						}
						if a.accessFilter != "" {
							chipButton(c, a.accessFilter+" ×", true).OnClick(func() { a.accessFilter = ""; a.filterCatalog(a.search, 0) })
						}
						for _, topic := range slices.Sorted(maps.Keys(a.tags)) {
							chipButton(c, topic+" ×", true).OnClick(func() { delete(a.tags, topic); a.filterCatalog(a.search, 0) })
						}
						linkButton(c, "Clear").OnClick(func() {
							a.tags = nil
							a.statusFilter = ""
							a.accessFilter = ""
							a.filterCatalog(a.search, 0)
						})
						ui.Box(c).Grow(1)
						total := fmt.Sprintf("%d problems", a.page.Total)
						if a.catalogLoading {
							total += "  ·  Updating…"
						}
						ui.Text(c, total).TextColor(t.TextMuted).FontSize(12).SingleLine()
					})
				}

				// Problem list
				ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
					a.tableHeader(c)
					slot := ui.Column(c.Key("catalog-slot")).Grow(1).FillWidth().MinHeight(0)
					availH := slot.LayoutBounds().H
					if availH > 0 && availH != a.catalogSlotH {
						a.catalogSlotH = availH
						c.AnimationFrame()
					}
					rowH := float32(46)
					if a.compact {
						rowH = 36
					}
					const rowGap = float32(2)
					const topPad = float32(6)
					step := rowH + rowGap
					hasRows := len(a.page.Problems) > 0 && a.catalogError == ""
					snappedH := float32(0)
					if hasRows && availH >= topPad+rowH {
						visibleRows := max(1, int((availH-topPad+rowGap)/step))
						snappedH = min(availH, topPad+float32(visibleRows)*step-rowGap)
					}
					a.catalogViewportH = snappedH
					slot.Clip().Children(func() {
						scroll := ui.Scroll(c.Key("catalog-scroll")).Scrollbars(false).TrackScroll(&a.catalogScroll).FillWidth()
						if snappedH > 0 {
							scroll.Height(snappedH)
						} else {
							scroll.Grow(1)
						}
						scroll.Children(func() {
							ui.Column(c).FillWidth().Padding(topPad, 0, 0, 0).Gap(rowGap).FocusGroup(ui.Vertical).Children(func() {
								if a.catalogError != "" {
									ui.Text(c, "Could not refresh problems").Bold().Padding(16)
									ui.Text(c, a.catalogError).TextColor(t.Danger).Padding(0, 16)
									quietButton(c, "Retry problems").Disabled(a.busy || time.Now().Before(a.catalogRetry)).OnClick(func() { a.list(a.listed, a.skip) })
								}
								if len(a.page.Problems) == 0 && a.catalogError == "" {
									title, detail := "No matching problems", "Try another title, number, or filter."
									if a.catalogLoading {
										title, detail = "Loading problems…", "Fetching the public catalog."
									}
									nullEmptyState(c, title, detail)
								}
								hoveredNow := 0
								for i, p := range a.page.Problems {
									if a.problemRow(c, i, p) {
										hoveredNow = i + 1
									}
								}
								scrolled := a.catalogScroll.Y != a.lastCatalogScrollY
								a.lastCatalogScrollY = a.catalogScroll.Y
								if hoveredNow != a.hoverRowPlus1 {
									a.hoverRowPlus1 = hoveredNow
									if a.listKeyboardNav && hoveredNow != 0 && !scrolled {
										a.listKeyboardNav = false
										a.selectedRow = -1
									}
								}
							})
						})
						if hasRows && a.catalogScroll.Y > 1 {
							ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(34).PassThrough().LinearGradient(ui.LinearGradient{
								Angle: 180,
								Stops: []ui.GradientStop{
									{Color: t.Surface, At: 0},
									{Color: t.Surface.Alpha(.88), At: 0.35},
									{Color: t.Surface.Alpha(.42), At: 0.72},
									{Color: t.Surface.Alpha(0), At: 1},
								},
							})
						}
						if hasRows && a.catalogScroll.MaxY > 0 && a.catalogScroll.Y > 1 && a.catalogScroll.Y < a.catalogScroll.MaxY-1 {
							bottomOffset := float32(0)
							if snappedH > 0 && availH > snappedH {
								bottomOffset = availH - snappedH
							}
							ui.Box(c).Absolute().Bottom(bottomOffset).Left(0).Right(0).Height(38).PassThrough().LinearGradient(ui.LinearGradient{
								Angle: 180,
								Stops: []ui.GradientStop{
									{Color: t.Surface.Alpha(0), At: 0},
									{Color: t.Surface.Alpha(.42), At: 0.28},
									{Color: t.Surface.Alpha(.88), At: 0.65},
									{Color: t.Surface, At: 1},
								},
							})
						}
					})
				})
			})
		})
		if totalPages > 1 {
			a.paginationBar(c, size, totalPages)
		} else {
			ui.Box(c).Height(22)
		}
	})
}

func (a *app) paginationBar(c *ui.Context, size, totalPages int) {
	t := c.Theme()
	current := a.skip/size + 1
	ui.Row(c).FillWidth().Height(72).Shrink(0).Padding(20, 32, 18, 32).Gap(4).Center().Children(func() {
		prevDisabled := a.skip == 0 || a.catalog == nil
		prev := ui.ButtonBase(c).Label("Previous").Height(34).Padding(0, 10).Radius(8).AlignItems(ui.Center).Disabled(prevDisabled)
		if !prevDisabled && prev.Hovered() {
			prev.Background(t.SurfaceHover)
		}
		prevColor := accentLinkColor(c)
		if prevDisabled {
			prevColor = t.TextMuted.Alpha(.55)
		}
		prev.Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconChevronLeft).FontSize(14).TextColor(prevColor)
				ui.Text(c, "Previous").FontSize(13).TextColor(prevColor).SingleLine()
			})
		}).OnClick(func() {
			if a.catalog != nil {
				a.filterCatalog(a.search, a.skip-size)
			}
		})

		pages := paginationPages(current, totalPages)
		for _, p := range pages {
			if p == 0 {
				ui.Box(c).MinWidth(34).Height(34).Center().Children(func() {
					ui.Text(c, "…").FontSize(13).TextColor(t.TextMuted)
				})
				continue
			}
			pageNum := p
			active := pageNum == current
			pageDisabled := !active && a.catalog == nil
			btn := ui.ButtonBase(c).Label(fmt.Sprint(pageNum)).MinWidth(34).Height(34).Padding(0, 10).Radius(8).Center().Disabled(pageDisabled)
			if active {
				btn.Background(t.Accent)
			} else if !pageDisabled && btn.Hovered() {
				btn.Background(t.SurfaceHover)
			}
			fg := t.Text
			weight := 400
			if active {
				fg = t.AccentText
				weight = 600
			} else if pageDisabled {
				fg = t.TextMuted.Alpha(.55)
			}
			btn.Children(func() { ui.Text(c, fmt.Sprint(pageNum)).FontSize(13).FontWeight(weight).TextColor(fg) })
			btn.OnClick(func() {
				if a.catalog != nil {
					a.filterCatalog(a.search, (pageNum-1)*size)
				}
			})
		}

		nextDisabled := current >= totalPages || a.catalog == nil
		next := ui.ButtonBase(c).Label("Next").Height(34).Padding(0, 10).Radius(8).AlignItems(ui.Center).Disabled(nextDisabled)
		if !nextDisabled && next.Hovered() {
			next.Background(t.SurfaceHover)
		}
		nextColor := accentLinkColor(c)
		if nextDisabled {
			nextColor = t.TextMuted.Alpha(.55)
		}
		next.Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Next").FontSize(13).TextColor(nextColor).SingleLine()
				ui.Icon(c, iconChevronRgt).FontSize(14).TextColor(nextColor)
			})
		}).OnClick(func() {
			if a.catalog != nil {
				a.filterCatalog(a.search, a.skip+size)
			}
		})
	})
}

func paginationPages(current, total int) []int {
	if total <= 5 {
		out := make([]int, total)
		for i := range out {
			out[i] = i + 1
		}
		return out
	}
	if current <= 3 {
		return []int{1, 2, 3, 0, total}
	}
	if current >= total-2 {
		return []int{1, 0, total - 2, total - 1, total}
	}
	return []int{1, 0, current - 1, current, current + 1, 0, total}
}

func (a *app) dailyView(c *ui.Context) {
	t := c.Theme()
	bg, border := dailyCardColors(c)
	hoverBg, hoverBorder := dailyCardHoverColors(c)
	topicBg, topicBorder := dailyTopicColors(c)
	date, _ := time.Parse("2006-01-02", a.daily.Date)
	title := "Loading today’s challenge…"
	if a.dailyError != "" {
		title = "Daily challenge unavailable"
	} else if a.daily.Problem.Slug != "" {
		title = strings.TrimSpace(a.daily.Problem.FrontendID) + ". " + strings.TrimSpace(a.daily.Problem.Title)
	}
	renderBody := func(hovered bool) {
		ui.Column(c).FillWidth().Gap(10).Children(func() {
			// Top meta row: calendar icon + Daily Challenge · Date
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconCalendar).FontSize(14).TextColor(accentLinkColor(c))
				caption := "Daily Challenge"
				if !date.IsZero() {
					caption += " · " + date.Format("Jan 2, Monday")
				}
				ui.Text(c, caption).FontSize(12.5).FontWeight(500).TextColor(t.TextMuted).SingleLine()
			})
			// Main row: Left (Title + Difficulty + ALL Topics), Right (lightweight action affordance or Retry)
			ui.Row(c).FillWidth().Gap(18).AlignItems(ui.End).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(10).Children(func() {
					titleColor := t.Text
					if hovered && a.daily.Problem.Slug != "" {
						titleColor = accentLinkColor(c)
					}
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, title).SingleLine().FontWeight(700).FontSize(18).TextColor(titleColor).Shrink(1).MinWidth(0)
						if a.daily.Problem.Slug != "" && a.solved[a.daily.Problem.Slug] {
							solvedCheckBadge(c)
						}
					})
					if a.daily.Problem.Slug != "" {
						ui.Row(c).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
							a.problemBadgesStyled(c, a.daily.Problem, 0, topicBg, topicBorder)
						})
					}
				})
				if a.dailyError != "" {
					outlineButton(c, "Retry", nil, 32).OnClick(a.refreshDaily)
				} else if a.daily.Problem.Slug != "" {
					ctaLabel := "Solve →"
					if a.solved[a.daily.Problem.Slug] {
						ctaLabel = "Review →"
					}
					ctaColor := accentLinkColor(c)
					if !hovered {
						ctaColor = ctaColor.Alpha(.88)
					}
					ui.Text(c, ctaLabel).FontSize(13).FontWeight(600).TextColor(ctaColor).Shrink(0).Padding(0, 2, 2, 0).SingleLine()
				}
			})
		})
	}
	if a.dailyError != "" || a.daily.Problem.Slug == "" {
		ui.Column(c).FillWidth().Padding(18, 22).Background(bg).Border(1, border).Radius(14).Children(func() {
			renderBody(false)
		})
		return
	}
	card := ui.ButtonBase(c.Key("daily-challenge-card")).Label("Daily Challenge: "+title).FillWidth().Padding(18, 22).Background(bg).Border(1, border).Radius(14).Disabled(a.busy)
	hovered := card.Hovered()
	if hovered {
		card.Background(hoverBg).BorderColor(hoverBorder)
	}
	applyPressFeedback(card, 0.992)
	card.Children(func() { renderBody(hovered) }).OnClick(func() {
		a.open(a.daily.Problem.Slug)
	})
}

func (a *app) problemBadges(c *ui.Context, p leetcode.Problem) {
	a.problemBadgesStyled(c, p, 0, chipBg(c), ui.Transparent)
}

func (a *app) problemBadgesStyled(c *ui.Context, p leetcode.Problem, maxTopics int, bg, border ui.Color) {
	difficultyBadge(c, p.Difficulty)
	if !a.showTopics {
		return
	}
	limit := len(p.Topics)
	overflow := 0
	if maxTopics > 0 && limit > maxTopics {
		overflow = limit - maxTopics
		limit = maxTopics
	}
	for _, tag := range p.Topics[:limit] {
		chip := ui.Text(c, tag.Name).FontSize(11.5).TextColor(chipText(c)).Padding(3, 10).Radius(11).Background(bg)
		if border != ui.Transparent {
			chip.Border(1, border)
		}
	}
	if overflow > 0 {
		chip := ui.Text(c, fmt.Sprintf("+%d", overflow)).FontSize(11.5).TextColor(chipText(c)).Padding(3, 10).Radius(11).Background(bg)
		if border != ui.Transparent {
			chip.Border(1, border)
		}
	}
}

func difficultyBadge(c *ui.Context, d string) {
	fg, bg := difficultyBadgeColors(c, d)
	ui.Text(c, displayDifficulty(d)).FontSize(12).FontWeight(600).TextColor(fg).Padding(3, 10).Radius(11).Background(bg)
}

func (a *app) tableHeader(c *ui.Context) {
	ui.Column(c).FillWidth().Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Height(36).Padding(0, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			if a.username != "" {
				ui.Box(c).Width(32)
			}
			a.sortButton(c, "#", 48, ui.Center)
			a.sortButtonGrow(c, "Title")
			if !a.hideAcceptance {
				a.sortButton(c, "Acceptance", 92, ui.Center)
			}
			if !a.hideDifficulty {
				a.sortButton(c, "Difficulty", 84, ui.Center)
			}
		})
		ui.Box(c.Key("table-header-divider")).FillWidth().Padding(0, 10).Children(func() {
			ui.Box(c).FillWidth().Height(1).Background(hairlineColor(c))
		})
	})
}

func (a *app) problemRow(c *ui.Context, idx int, p leetcode.Problem) bool {
	t := c.Theme()
	height := float32(46)
	if a.compact {
		height = 36
	}
	title := strings.TrimSpace(p.Title)
	b := ui.ButtonBase(c.Key(p.Slug)).Label(p.FrontendID+". "+title).FillWidth().Height(height).Padding(0, 16).Radius(10).Disabled(a.busy)
	hovered := b.Hovered()
	highlighted := false
	if a.listKeyboardNav {
		highlighted = idx == a.selectedRow
	} else {
		highlighted = hovered || idx == a.selectedRow
	}
	if highlighted {
		b.Background(t.SurfaceHover)
	}
	b.Transition(smoothColorTransition)
	b.Children(func() {
		ui.Row(c).Fill().Gap(8).AlignItems(ui.Center).Children(func() {
			if a.username != "" {
				ui.Row(c).Width(32).Center().Children(func() {
					if a.solved[p.Slug] {
						solvedCheckBadge(c)
					}
				})
			}
			ui.Text(c, p.FrontendID).Width(48).TextAlign(ui.Center).FontSize(13).TextColor(t.TextMuted).SingleLine()
			textColor := t.Text
			if p.Paid && !a.premium {
				textColor = t.TextMuted
			}
			ui.Row(c).Grow(1).MinWidth(0).Padding(0, 8, 0, 0).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(13.5).FontWeight(500).TextColor(textColor).Shrink(1).MinWidth(0).SingleLine()
				if p.Paid {
					label := "Premium"
					if a.site == leetcode.CN {
						label = "会员"
					}
					pfg, pbg := premiumBadgeColors(c)
					ui.Row(c).Padding(2, 9).Gap(4).Radius(10).Background(pbg).AlignItems(ui.Center).Shrink(0).Children(func() {
						ui.Icon(c, iconLock).FontSize(11).TextColor(pfg)
						ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(pfg).SingleLine()
					})
				}
			})
			if !a.hideAcceptance {
				v := "—"
				if p.Acceptance != nil {
					v = fmt.Sprintf("%.1f%%", *p.Acceptance)
				}
				ui.Text(c, v).Width(92).TextAlign(ui.Center).FontSize(13).TextColor(t.TextMuted).SingleLine()
			}
			if !a.hideDifficulty {
				ui.Row(c).Width(84).Center().Children(func() { difficultyBadge(c, p.Difficulty) })
			}
		})
	})
	b.OnClick(func() { a.selectedRow = -1; a.listKeyboardNav = false; a.open(p.Slug) })
	address := a.site.URL() + "/problems/" + url.PathEscape(p.Slug) + "/"
	b.ContextMenu(func(m *ui.Menu) {
		if m.Item("Open").Chosen() {
			a.open(p.Slug)
		}
		if m.Item("Open in browser").Chosen() {
			mygo.Shell.OpenExternal(address)
		}
		if m.Item("Copy link").Chosen() {
			c.WriteClipboard(address)
		}
		if m.Item("Copy number and title").Chosen() {
			c.WriteClipboard(p.FrontendID + ". " + title)
		}
	})
	return hovered
}

func (a *app) sortButton(c *ui.Context, key string, w float32, align ui.Align) {
	t := c.Theme()
	label := key
	fg := t.TextMuted
	if a.sortColumn == key {
		fg = t.Text
		if a.descending {
			label += " ↓"
		} else {
			label += " ↑"
		}
	}
	b := ui.ButtonBase(c).Label(key).Height(28).Disabled(a.catalog == nil)
	if w > 0 {
		b.Width(w)
	} else {
		b.Grow(1).Padding(0, 8, 0, 0)
	}
	if b.Hovered() {
		fg = t.Text
	}
	b.Children(func() {
		ui.Row(c).Fill().Justify(align).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(fg).SingleLine()
		})
	}).OnClick(func() { a.toggleSort(key) })
}

func (a *app) sortButtonGrow(c *ui.Context, key string) {
	a.sortButton(c, key, 0, ui.Start)
}
