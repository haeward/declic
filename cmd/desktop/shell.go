package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
)

func (a *app) windowNavControls(c *ui.Context) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		titleBarIconButton(c, "Toggle sidebar", "Toggle sidebar ("+shortcutBadge("B")+")", iconSidebar, false).OnClick(a.toggleSidebar)
		ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
			titleBarIconButton(c, "Go back", "Go back ("+shortcutBadge("[")+")", iconBack, !a.canGoBack()).OnClick(a.goBack)
			titleBarIconButton(c, "Go forward", "Go forward ("+shortcutBadge("]")+")", iconForward, !a.canGoForward()).OnClick(a.goForward)
		})
	})
}

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	collapsed := a.sidebarCollapsed
	width := float32(200)
	if collapsed {
		width = 56
	}
	item := func(icon *ui.SVG, label string, selected bool) ui.Element {
		if !collapsed {
			return navItem(c, icon, label, selected)
		}
		b := ui.ButtonBase(c).Label(label).Size(36, 36).Center().Radius(9)
		if selected {
			b.Background(sidebarActiveColor(c))
		} else if b.Hovered() {
			b.Background(sidebarActiveColor(c).Alpha(.55))
		}
		fg := t.TextMuted
		if selected {
			fg = t.Text
		}
		b.Children(func() { ui.Icon(c, icon).FontSize(16).TextColor(fg) })
		return attachTooltip(c, b, label, tipRight)
	}
	ui.Column(c.Key("navigation")).Width(width).Shrink(0).FillHeight().Padding(6, 10, 14, 10).Gap(3).Background(sidebarColor(c)).Children(func() {
		a.sidebarSiteBadge(c, collapsed)
		item(iconProblems, "Problems", a.screen != "workspace").Disabled(a.busy).OnClick(func() { a.navigate("problems") })
		if a.problem.ID != "" {
			if !collapsed {
				ui.Text(c, "Current problem").FontSize(11).TextColor(t.TextMuted).Padding(14, 10, 4, 10)
			}
			curLabel := strings.TrimSpace(a.problem.FrontendID) + ". " + strings.TrimSpace(a.problem.Title)
			item(iconCode, curLabel, a.screen == "workspace").Disabled(a.busy).OnClick(func() { a.navigate("workspace") })
		}
		if !collapsed {
			var others []leetcode.Problem
			for _, rp := range a.recentProblems {
				if rp.Slug != "" && rp.Slug != a.problem.Slug {
					others = append(others, rp)
					if len(others) >= 5 {
						break
					}
				}
			}
			if len(others) > 0 {
				ui.Text(c, "Recent").FontSize(11).TextColor(t.TextMuted).Padding(14, 10, 4, 10)
				for _, rp := range others {
					p := rp
					label := strings.TrimSpace(p.FrontendID) + ". " + strings.TrimSpace(p.Title)
					item(iconCode, label, false).Disabled(a.busy).OnClick(func() { a.open(p.Slug) })
				}
			}
		}
		ui.Box(c).Grow(1)
		display := a.displayName()
		if display == "" {
			display = "Sign in"
		}
		var anchor ui.Element
		if a.username == "" {
			anchor = item(iconAccount, display, false)
		} else if collapsed {
			b := ui.ButtonBase(c).Label(display).Size(36, 36).Center().Radius(9)
			if a.accountPopup {
				b.Background(sidebarActiveColor(c))
			} else if b.Hovered() {
				b.Background(sidebarActiveColor(c).Alpha(.55))
			}
			b.Children(func() { a.renderAvatar(c, 22, 11) })
			anchor = attachTooltip(c, b, display, tipRight)
		} else {
			b := ui.ButtonBase(c).Label(display).FillWidth().Height(34).Padding(0, 10).Radius(8)
			if a.accountPopup {
				b.Background(sidebarActiveColor(c))
			} else if b.Hovered() {
				b.Background(sidebarActiveColor(c).Alpha(.55))
			}
			applyPressFeedback(b, 0.985)
			b.Children(func() {
				ui.Row(c).Fill().Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(22).Center().Shrink(0).Children(func() {
						a.renderAvatar(c, 20, 10.5)
					})
					fg := t.TextMuted
					if a.accountPopup {
						fg = t.Text
					}
					ui.Text(c, display).FontSize(13).TextColor(fg).Grow(1).SingleLine()
				})
			})
			anchor = b
		}
		anchor.OnClick(func() {
			if a.username == "" {
				a.openLogin()
			} else {
				a.accountPopup = !a.accountPopup
			}
		})
		if a.accountPopup && a.username != "" {
			ui.PopoverBase(c, anchor, &a.accountPopup, func(panel ui.Element) {
				if collapsed {
					panel.AttachTo(anchor, ui.AnchorBottomEnd, ui.AnchorBottomStart).Margin(0, 0, 0, 8)
				} else {
					panel.AttachTo(anchor, ui.AnchorTopStart, ui.AnchorBottomStart).Margin(0, 0, 8, 0)
				}
				panel.Width(180).Padding(8).Gap(2).Radius(12).Background(t.Surface).Border(1, t.Border.Alpha(.55)).Shadow(0, 10, 28, 0, ui.RGBA(0, 0, 0, .18))
				ui.Row(c).FillWidth().Gap(10).Padding(4, 6, 6, 6).AlignItems(ui.Center).Children(func() {
					a.renderAvatar(c, 32, 13)
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, display).FontSize(13).FontWeight(600).TextColor(t.Text).SingleLine()
						if a.expired {
							ui.Text(c, "Session expired").FontSize(11).TextColor(t.Danger).SingleLine()
						} else {
							ui.Text(c, string(a.site)).FontSize(11).TextColor(t.TextMuted.Alpha(.75)).SingleLine()
						}
					})
				})
				ui.Box(c).FillWidth().Height(1).Background(hairlineColor(c)).Margin(2, 0, 3, 0)
				if a.expired {
					popoverIconRow(c, iconAccount, "Sign in again", t.Accent).OnClick(func() { a.accountPopup = false; a.openLogin() })
				}
				profileURL := a.site.URL() + "/u/" + url.PathEscape(a.username) + "/"
				popoverIconRow(c, iconExternal, "Open profile", t.Text).OnClick(func() {
					a.accountPopup = false
					mygo.Shell.OpenExternal(profileURL)
				})
				signOutBtn := ui.ButtonBase(c).Label("Sign out").FillWidth().Height(30).Padding(0, 8).Radius(7).Disabled(a.busy)
				signOutColor := t.TextMuted
				if signOutBtn.Hovered() {
					signOutBtn.Background(t.Danger.Alpha(.10))
					signOutColor = t.Danger
				}
				signOutBtn.Children(func() {
					ui.Row(c).Fill().Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconSignOut).FontSize(13.5).TextColor(signOutColor)
						ui.Text(c, "Sign out").FontSize(12.5).TextColor(signOutColor).SingleLine()
					})
				}).OnClick(func() { a.accountPopup = false; a.logout() })
			})
		}
		item(iconGear, "Settings", false).OnClick(func() { a.openSettings("") })
	})
}

func (a *app) sidebarSiteBadge(c *ui.Context, collapsed bool) {
	t := c.Theme()
	canSwitch := a.screen != "workspace" && a.username == "" && !a.busy
	if !canSwitch {
		a.sitePopup = false
	}
	brandIcon := iconSiteLight
	if t.Dark {
		brandIcon = iconSiteDark
	}
	siteTitle := "LeetCode"
	if a.site == leetcode.CN {
		siteTitle = "LeetCode CN"
	}
	readOnlyTip := ""
	if a.username != "" {
		readOnlyTip = "Signed in on " + string(a.site) + ". Sign out to switch sites."
	} else if a.screen == "workspace" {
		readOnlyTip = "Return to Problems to switch sites."
	}
	var anchor ui.Element
	if collapsed {
		b := ui.ButtonBase(c).Label("Site: "+string(a.site)).Size(36, 36).Center().Radius(10).Margin(2, 0, 10, 0).Disabled(a.busy)
		if canSwitch {
			applyPressFeedback(b, 0.94)
		}
		b.Children(func() {
			ui.Image(c, brandIcon).Size(26, 26).Fit(ui.Contain)
		})
		tip := siteTitle + " · " + string(a.site)
		if canSwitch {
			tip += " · Click to switch"
		} else if readOnlyTip != "" {
			tip += " · " + readOnlyTip
		}
		anchor = attachTooltip(c, b, tip, tipRight)
	} else {
		b := ui.ButtonBase(c).Label("Site: "+string(a.site)).FillWidth().Padding(6, 10).Radius(10).Margin(2, 0, 10, 0).Disabled(a.busy)
		hovered := b.Hovered()
		if canSwitch {
			applyPressFeedback(b, 0.98)
		}
		b.Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Image(c, brandIcon).Size(30, 30).Fit(ui.Contain).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Text(c, siteTitle).FontSize(14).FontWeight(700).TextColor(t.Text).SingleLine()
						if !canSwitch && a.username != "" {
							dotColor := t.Success
							if a.expired {
								dotColor = t.Danger
							}
							ui.Box(c).Size(12, 12).Radius(6).Background(dotColor.Alpha(.16)).Center().Shrink(0).Children(func() {
								ui.Box(c).Size(5, 5).Radius(2.5).Background(dotColor)
							})
						}
					})
					ui.Text(c, string(a.site)).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
				})
				if canSwitch {
					chevColor := t.TextMuted.Alpha(.72)
					if hovered || a.sitePopup {
						chevColor = t.Text
					}
					rotTarget := float32(0)
					if a.sitePopup {
						rotTarget = 180
					}
					chev := ui.Icon(c, iconChevronDown).FontSize(13.5).TextColor(chevColor).Shrink(0)
					chev.Rotate(chev.AnimateTo("site-chev-rot", rotTarget, ui.Snappy))
				}
			})
		})
		if !canSwitch && readOnlyTip != "" {
			attachTooltip(c, b, readOnlyTip, tipBottom)
		}
		anchor = b
	}
	anchor.OnClick(func() {
		if canSwitch {
			a.sitePopup = !a.sitePopup
		}
	})
	if a.sitePopup && canSwitch {
		ui.PopoverBase(c, anchor, &a.sitePopup, func(panel ui.Element) {
			if collapsed {
				panel.AttachTo(anchor, ui.AnchorTopEnd, ui.AnchorTopStart).Margin(0, 0, 0, 8)
			} else {
				panel.AttachTo(anchor, ui.AnchorBottomStart, ui.AnchorTopStart).Margin(4, 0, 0, 0)
			}
			panel.Width(196).Padding(8).Gap(2).Radius(12).Background(t.Surface).Border(1, t.Border.Alpha(.55)).Shadow(0, 10, 28, 0, ui.RGBA(0, 0, 0, .18))
			ui.Text(c, "Switch site").FontSize(11.5).FontWeight(600).TextColor(t.TextMuted).Padding(2, 8, 5, 8)
			for _, opt := range []struct {
				site  leetcode.Site
				title string
			}{
				{leetcode.COM, "LeetCode"},
				{leetcode.CN, "LeetCode CN"},
			} {
				site := opt.site
				active := a.site == site
				row := ui.ButtonBase(c).Label(string(site)).FillWidth().Height(38).Padding(0, 8).Radius(8)
				if active {
					row.Background(activeChipBg(c))
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				applyPressFeedback(row, 0.985)
				row.Children(func() {
					ui.Row(c).Fill().Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
							fg := t.Text
							weight := 500
							if active {
								fg = accentLinkColor(c)
								weight = 600
							}
							ui.Text(c, opt.title).FontSize(12.5).FontWeight(weight).TextColor(fg).SingleLine()
							ui.Text(c, string(site)).FontSize(11).TextColor(t.TextMuted).SingleLine()
						})
						if active {
							ui.Icon(c, iconCheck).FontSize(13).TextColor(accentLinkColor(c))
						}
					})
				}).OnClick(func() {
					a.sitePopup = false
					a.switchSite(site)
				})
			}
		})
	}
	if collapsed {
		insetHairline(c, sidebarDividerColor(c), 0, 8, 10)
	} else {
		insetHairline(c, sidebarDividerColor(c), 0, 10, 10)
	}
}

func (a *app) renderAvatar(c *ui.Context, size, fontSize float32) {
	if a.avatar != nil {
		ui.Box(c).Size(size, size).Radius(size / 2).Clip().Shrink(0).Children(func() {
			ui.Image(c, a.avatar).Size(size, size).Fit(ui.Cover)
		})
		return
	}
	avatarCircle(c, a.displayName(), size, fontSize)
}

func avatarCircle(c *ui.Context, name string, size, fontSize float32) {
	initial := "?"
	if r := []rune(name); len(r) > 0 {
		initial = strings.ToUpper(string(r[0]))
	}
	ui.Box(c).Size(size, size).Radius(size / 2).Shrink(0).Center().Background(c.Theme().Accent).Children(func() {
		ui.Text(c, initial).FontSize(fontSize).FontWeight(600).TextColor(c.Theme().AccentText)
	})
}

func popoverMenuRow(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).FillWidth().Height(32).Padding(0, 10).Radius(8)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Row(c).Fill().AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(13).TextColor(t.Text).SingleLine()
		})
	})
	return b
}

func popoverIconRow(c *ui.Context, icon *ui.SVG, label string, fg ui.Color) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).FillWidth().Height(30).Padding(0, 8).Radius(7)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Row(c).Fill().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon).FontSize(13.5).TextColor(t.TextMuted)
			ui.Text(c, label).FontSize(12.5).TextColor(fg).SingleLine()
		})
	})
	return b
}

func (a *app) premiumDialog(c *ui.Context) {
	t := c.Theme()
	p := a.restricted
	pfg, pbg := premiumBadgeColors(c)
	ui.DialogBase(c, &a.premiumModal, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .28))
		panel.Background(t.Surface).Width(440).Padding(26, 28, 24, 28).Radius(16).Border(1, t.Border.Alpha(.5)).Shadow(0, 16, 42, 0, ui.RGBA(0, 0, 0, .24)).Children(func() {
			ui.Column(c).FillWidth().Gap(16).Children(func() {
				ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(44, 44).Radius(22).Background(pbg).Center().Shrink(0).Children(func() {
						ui.Icon(c, iconLock).FontSize(20).TextColor(pfg)
					})
					ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
						ui.Text(c, "Premium problem").FontSize(17).Bold().TextColor(t.Text)
						if p.Title != "" {
							title := p.Title
							if p.FrontendID != "" {
								title = p.FrontendID + ". " + title
							}
							ui.Text(c, title).FontSize(13).TextColor(t.TextMuted).SingleLine()
						}
					})
				})
				msg := fmt.Sprintf("This problem requires an active LeetCode Premium subscription on %s to view and submit.", a.site)
				if a.username == "" {
					msg = fmt.Sprintf("This is a LeetCode Premium problem. Sign in with a subscribed %s account, or open it in your browser.", a.site)
				}
				ui.Text(c, msg).FontSize(13).TextColor(t.TextMuted)
				ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Padding(4, 0, 0, 0).Children(func() {
					outlineButton(c, "Close", nil, 34).OnClick(func() { a.premiumModal = false })
					if p.Slug != "" {
						problemURL := a.site.URL() + "/problems/" + url.PathEscape(p.Slug) + "/"
						if a.username == "" {
							outlineButton(c, "Open on "+string(a.site), iconExternal, 34).OnClick(func() {
								a.premiumModal = false
								mygo.Shell.OpenExternal(problemURL)
							})
							primaryActionButton(c, "Sign in", nil, 34).OnClick(func() {
								a.premiumModal = false
								a.openLogin()
							})
						} else {
							primaryActionButton(c, "Open on "+string(a.site), iconExternal, 34).OnClick(func() {
								a.premiumModal = false
								mygo.Shell.OpenExternal(problemURL)
							})
						}
					}
				})
			})
		})
	})
}

func (a *app) switchSite(site leetcode.Site) {
	if a.busy || a.username != "" || site == a.site {
		return
	}
	a.save(func() { a.resetDocument(site); a.list("", 0) })
}

func (a *app) quickOpenDialog(c *ui.Context) {
	t := c.Theme()
	ui.DialogBase(c, &a.showQuickOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .24))
		panel.Background(t.Surface).Width(560).Padding(22).Radius(14).Border(1, t.Border.Alpha(.5)).Shadow(0, 12, 36, 0, ui.RGBA(0, 0, 0, .22)).Children(func() {
			ui.Column(c).FillWidth().Gap(14).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Quick open").FontSize(17).Bold().Grow(1)
					quietButton(c, "Close").OnClick(func() { a.showQuickOpen = false })
				})
				input := ui.TextInput(c.Key("quick-search"), &a.quickSearch).Label("Quick open search").Placeholder("Problem title or number").FillWidth().Disabled(a.busy)
				if a.quickFocusPending {
					input.Focus()
					a.quickFocusPending = false
				}
				if a.quickSearch != a.quickListed || input.Shortcut(0, ui.KeyEnter) {
					a.quickListed = a.quickSearch
					a.searchQuickOpen()
				}
				if a.quickError != "" {
					ui.Text(c, a.quickError).TextColor(t.Danger)
				}
				ui.Scroll(c.Key("quick-results")).Scrollbars(false).FillWidth().Height(280).Children(func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						if a.quickPage.Problems == nil {
							ui.Text(c, "Search the current site. Your editor stays open.").TextColor(t.TextMuted).Padding(12)
						}
						if a.quickPage.Problems != nil && len(a.quickPage.Problems) == 0 {
							ui.Text(c, "No matching problems").Padding(12)
						}
						for _, p := range a.quickPage.Problems {
							popoverMenuRow(c, fmt.Sprintf("%s. %s", p.FrontendID, p.Title)).Disabled(a.busy).OnClick(func() { a.showQuickOpen = false; a.open(p.Slug) })
						}
					})
				})
			})
		})
	})
}

func (a *app) searchQuickOpen() {
	query := strings.TrimSpace(a.quickSearch)
	a.quickPage = leetcode.Page{Problems: []leetcode.Problem{}}
	for _, p := range a.catalog {
		if catalogMatch(p, query) || strings.Contains(strings.ToLower(p.Slug), strings.ToLower(query)) {
			a.quickPage.Problems = append(a.quickPage.Problems, p)
			if len(a.quickPage.Problems) == 50 {
				break
			}
		}
	}
	a.quickError = ""
	if a.catalog == nil {
		a.quickError = "The site catalog is unavailable."
	}
}

func (a *app) openQuickOpen() { a.showQuickOpen, a.quickFocusPending = true, true }
