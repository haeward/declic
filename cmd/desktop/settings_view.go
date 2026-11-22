package main

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
)

var settingsSections = []string{
	"General",
	"Appearance",
	"Problems",
	"Workspace",
	"Editor",
	"Run & Submit",
	"Shortcuts",
	"Account & Network",
	"Storage",
}

func (a *app) settingsView(c *ui.Context) {
	t := c.Theme()
	section := a.settingsSection
	if section == "" {
		section = "General"
	}
	ui.Row(c).Grow(1).FillWidth().MinHeight(0).Background(t.Surface).Children(func() {
		ui.Column(c).Width(196).Shrink(0).FillHeight().Padding(18, 12).Gap(3).Background(settingsSidebarColor(c)).Children(func() {
			for _, s := range settingsSections {
				selected := section == s
				b := ui.ButtonBase(c).Label(s).FillWidth().Height(34).Padding(0, 12).Radius(8)
				if selected {
					b.Background(sidebarActiveColor(c))
				} else if b.Hovered() {
					b.Background(sidebarActiveColor(c).Alpha(.55))
				}
				fg, weight := t.TextMuted, 400
				if selected {
					fg, weight = t.Text, 600
				}
				b.Children(func() {
					ui.Row(c).Fill().AlignItems(ui.Center).Children(func() {
						ui.Text(c, s).FontSize(13).FontWeight(weight).TextColor(fg).SingleLine()
					})
				}).OnClick(func() {
					a.settingsSection = s
					if s == "Storage" {
						a.refreshStorageStats()
					}
				})
			}
		})
		ui.Scroll(c.Key("settings-content")).Scrollbars(false).Grow(1).FillHeight().Children(func() {
			ui.Column(c).FillWidth().MaxWidth(580).Padding(28, 36).Gap(8).Children(func() {
				ui.Text(c, section).FontSize(20).Bold().Padding(0, 0, 12, 0)
				switch section {
				case "General":
					settingRow(c, "Browsing site", "Public catalogs and drafts are kept separate for each site", func() { a.siteControl(c) })
					if a.username != "" {
						ui.Text(c, "Sign out before connecting another site or account.").FontSize(12).TextColor(t.TextMuted)
					}
					settingRow(c, "Open on launch", "Choose where Declic starts when you open the app", func() {
						segmentedTrack(c, func() {
							for _, opt := range []struct {
								label, val string
							}{{"Problems", ""}, {"Last workspace", "last_workspace"}} {
								active := a.openOnLaunch == opt.val
								fg := t.TextMuted
								if active {
									fg = t.Text
								}
								v := opt.val
								segmentItem(c, opt.label, active, fg, 28).OnClick(func() { a.openOnLaunch = v })
							}
						})
					})
					settingRow(c, "Restore session on launch", "Verify your saved site session automatically at startup", func() {
						onOffSegmented(c, !a.noRestoreSession, func(on bool) { a.noRestoreSession = !on })
					})
				case "Shortcuts":
					settingValueRow(c, "Toggle sidebar", "Expand or collapse the navigation sidebar", shortcutDisplay("B"))
					settingValueRow(c, "Go back / forward", "Navigate recent pages and problems", shortcutDisplay("[")+"  /  "+shortcutDisplay("]"))
					settingValueRow(c, "Quick open", "Open any problem by title, number, or slug", shortcutDisplay("P"))
					settingValueRow(c, "Search problems", "Focus the catalog search field", shortcutDisplay("F"))
					settingValueRow(c, "Run code", "Run current code against active testcases", shortcutDisplay("↩"))
					settingValueRow(c, "Submit code", "Submit current code to LeetCode", shiftShortcutDisplay("↩"))
					settingValueRow(c, "Toggle test panel", "Expand or collapse testcases and results", shortcutDisplay("J"))
					settingValueRow(c, "Focus editor", "Maximize or restore the code editor pane", shortcutDisplay("\\"))
					settingValueRow(c, "Save draft", "Save current code to local disk", shortcutDisplay("S"))
					settingValueRow(c, "Settings", "Open the Settings window", shortcutDisplay(","))
					settingValueRow(c, "Close Settings", "Dismiss the Settings window", "Esc  /  "+shortcutDisplay("W"))
				case "Appearance":
					themeDetail := "Follows system appearance by default"
					if runtime.GOOS == "darwin" {
						themeDetail = "Follows macOS by default"
					}
					settingRow(c, "Theme", themeDetail, func() {
						segmentedTrack(c, func() {
							for _, mode := range []string{"System", "Light", "Dark"} {
								active := a.appearance == mode || (a.appearance == "" && mode == "System")
								fg := t.TextMuted
								if active {
									fg = t.Text
								}
								segmentItem(c, mode, active, fg, 28).OnClick(func() {
									a.appearance = mode
									a.syncNativeTheme()
									a.renderDescription()
								})
							}
						})
					})
					if runtime.GOOS == "darwin" {
						a.dockIconPicker(c, mygo.App.Dock.SetIcon)
					}
					settingRow(c, "Contrast", "Raise it for easier reading", func() {
						ui.Slider(c, &a.contrast, 0, 1).Width(160)
					})
					settingRow(c, "Density", "Row height in lists", func() {
						segmentedTrack(c, func() {
							for _, opt := range []struct {
								label   string
								compact bool
							}{{"Compact", true}, {"Comfortable", false}} {
								active := a.compact == opt.compact
								fg := t.TextMuted
								if active {
									fg = t.Text
								}
								segmentItem(c, opt.label, active, fg, 28).OnClick(func() { a.compact = opt.compact })
							}
						})
					})
					settingRow(c, "Description font size", "Applies to problem text", func() {
						size := max(13, a.descriptionSize)
						segmentedTrack(c, func() {
							segmentItem(c, "−", false, t.TextMuted, 28).Label("Decrease font size").Disabled(size <= 13).OnClick(func() {
								a.descriptionSize = max(13, size-1)
								a.renderDescription()
							})
							segmentItem(c, fmt.Sprintf("%d pt", size), true, t.Text, 28)
							segmentItem(c, "+", false, t.TextMuted, 28).Label("Increase font size").Disabled(size >= 19).OnClick(func() {
								a.descriptionSize = min(19, size+1)
								a.renderDescription()
							})
						})
					})
					settingRow(c, "Show topic tags", "Turn off to avoid spoilers", func() {
						segmentedTrack(c, func() {
							for _, opt := range []struct {
								label string
								show  bool
							}{{"On", true}, {"Off", false}} {
								active := a.showTopics == opt.show
								fg := t.TextMuted
								if active {
									fg = t.Text
								}
								segmentItem(c, opt.label, active, fg, 28).OnClick(func() { a.showTopics = opt.show })
							}
						})
					})
				case "Problems":
					settingRow(c, "Acceptance column", "Show acceptance rate in the problem list", func() {
						onOffSegmented(c, !a.hideAcceptance, func(on bool) { a.hideAcceptance = !on })
					})
					settingRow(c, "Difficulty column", "Show difficulty pills in the problem list", func() {
						onOffSegmented(c, !a.hideDifficulty, func(on bool) { a.hideDifficulty = !on })
					})
					settingRow(c, "Hide Premium problems", "Filter out subscriber-only problems", func() {
						onOffSegmented(c, a.hidePremium, func(on bool) {
							if a.hidePremium != on {
								a.hidePremium = on
								a.filterCatalog(a.search, 0)
							}
						})
					})
					settingRow(c, "Problems per page", "Number of rows shown per page", func() {
						segmentedTrack(c, func() {
							for _, size := range []int{20, 50, 100} {
								active := size == a.perPage()
								fg := t.TextMuted
								if active {
									fg = t.Text
								}
								segmentItem(c, fmt.Sprint(size), active, fg, 28).OnClick(func() {
									a.pageSize = size
									a.filterCatalog(a.search, 0)
								})
							}
						})
					})
				case "Editor":
					settingRow(c, "Font size", "Monospace text size in the code editor", func() {
						size := a.editorFontSize
						if size < 11 || size > 18 {
							size = 13
						}
						segmentedTrack(c, func() {
							segmentItem(c, "−", false, t.TextMuted, 28).Label("Decrease editor font size").Disabled(size <= 11).OnClick(func() {
								a.editorFontSize = max(11, size-1)
							})
							segmentItem(c, fmt.Sprintf("%d pt", size), true, t.Text, 28)
							segmentItem(c, "+", false, t.TextMuted, 28).Label("Increase editor font size").Disabled(size >= 18).OnClick(func() {
								a.editorFontSize = min(18, size+1)
							})
						})
					})
					settingRow(c, "Tab width", "Auto uses each language's convention (tabs for Go, 2 or 4 spaces elsewhere)", func() {
						tab := a.editorTabSize
						if tab != 0 && tab != 2 && tab != 4 {
							tab = 0
						}
						segmentedTrack(c, func() {
							for _, opt := range []struct {
								label string
								val   int
							}{{"Auto", 0}, {"2 spaces", 2}, {"4 spaces", 4}} {
								active := tab == opt.val
								fg := t.TextMuted
								if active {
									fg = t.Text
								}
								v := opt.val
								segmentItem(c, opt.label, active, fg, 28).OnClick(func() { a.editorTabSize = v })
							}
						})
					})
					settingRow(c, "Wrap long lines", "Soft-wrap lines that exceed the editor width", func() {
						onOffSegmented(c, a.code.SoftWrap, func(on bool) { a.code.SoftWrap = on })
					})
					settingRow(c, "Auto-close brackets", "Automatically close brackets and quotes while typing", func() {
						onOffSegmented(c, !a.code.NoAutoClose, func(on bool) { a.code.NoAutoClose = !on })
					})
					settingValueRow(c, "Autosave", "Automatically saved after you pause typing", "On")
					settingValueRow(c, "Initialization", "A local save is not a LeetCode submission", "Local draft, then template")
				case "Workspace":
					settingValueRow(c, "Toggle test panel", "Show or hide testcases and judge results", shortcutDisplay("J"))
					settingValueRow(c, "Layout", "Switches to tabbed panes in narrow windows", "Adaptive split view")
				case "Run & Submit":
					settingValueRow(c, "Execution provider", "Uses your verified official session", "LeetCode website")
				case "Account & Network":
					if a.username == "" {
						settingRow(c, "LeetCode account", "Connect an account through the official sign-in page", func() {
							primaryActionButton(c, "Sign in to LeetCode", nil, 34).Disabled(a.busy).OnClick(func() { a.openLogin() })
						})
					} else {
						accountMeta := string(a.site) + " · Signed in"
						if a.nickname != "" && a.nickname != a.username {
							accountMeta = "@" + a.username + " · " + accountMeta
						}
						settingRow(c, a.displayName(), accountMeta, func() {
							outlineButton(c, "Sign out", nil, 34).Disabled(a.busy).OnClick(a.logout)
						})
					}
				case "Storage":
					a.ensureStorageStats()
					draftDetail := "Stored on this device, separated by site and identity"
					if a.storageLoaded {
						if a.storageStats.draftCount == 1 {
							draftDetail = fmt.Sprintf("1 draft · %s · Separated by site and identity", formatByteSize(a.storageStats.draftBytes))
						} else if a.storageStats.draftCount > 1 {
							draftDetail = fmt.Sprintf("%d drafts · %s · Separated by site and identity", a.storageStats.draftCount, formatByteSize(a.storageStats.draftBytes))
						} else {
							draftDetail = "No local drafts yet · Separated by site and identity"
						}
					}
					revealLabel := "Open folder"
					if runtime.GOOS == "darwin" {
						revealLabel = "Show in Finder"
					}
					settingRow(c, "Code drafts", draftDetail, func() {
						outlineButton(c, revealLabel, iconExternal, 32).Disabled(a.store.Root == "").OnClick(a.openWorkspaceFolder)
					})
					cacheDetail := "Replaceable catalog, problem detail, and editorial metadata"
					if a.storageLoaded {
						if a.storageStats.cacheBytes > 0 {
							cacheDetail = fmt.Sprintf("%s · Replaceable catalog, problem detail, and editorial metadata", formatByteSize(a.storageStats.cacheBytes))
						} else {
							cacheDetail = "Empty · Replaceable catalog, problem detail, and editorial metadata"
						}
					}
					clearLabel := "Clear caches"
					if a.storageClearing {
						clearLabel = "Clearing…"
					}
					settingRow(c, "Public caches", cacheDetail, func() {
						outlineButton(c, clearLabel, iconTrash, 32).Disabled(a.storageClearing || a.busy || a.store.Root == "" || (a.storageLoaded && a.storageStats.cacheBytes == 0)).OnClick(a.clearPublicCachesAsync)
					})
					if a.storageNotice != "" {
						ui.Text(c, a.storageNotice).FontSize(12).TextColor(t.TextMuted).Padding(4, 0, 0, 0)
					}
				}
			})
		})
	})
}

func settingRow(c *ui.Context, title, detail string, control func()) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinHeight(54).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, title).FontSize(13).TextColor(t.Text)
			if detail != "" {
				ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted)
			}
		})
		control()
	})
}

func settingValueRow(c *ui.Context, title, detail, value string) {
	settingRow(c, title, detail, func() {
		ui.Text(c, value).FontSize(12.5).TextColor(c.Theme().TextMuted)
	})
}

func onOffSegmented(c *ui.Context, value bool, set func(bool)) {
	t := c.Theme()
	segmentedTrack(c, func() {
		for _, opt := range []struct {
			label string
			on    bool
		}{{"On", true}, {"Off", false}} {
			active := value == opt.on
			fg := t.TextMuted
			if active {
				fg = t.Text
			}
			segmentItem(c, opt.label, active, fg, 28).OnClick(func() { set(opt.on) })
		}
	})
}

func (a *app) siteControl(c *ui.Context) {
	t := c.Theme()
	segmentedTrack(c, func() {
		for _, site := range []leetcode.Site{leetcode.COM, leetcode.CN} {
			active := a.site == site
			fg := t.TextMuted
			if active {
				fg = t.Text
			}
			segmentItem(c, string(site), active, fg, 28).Disabled(a.busy || a.username != "").OnClick(func() { a.switchSite(site) })
		}
	})
}

func (a *app) openSettings(section string) {
	if section != "" {
		a.settingsSection = section
	}
	if a.settingsWindow != nil {
		a.settingsWindow.Update(func() {})
		a.settingsWindow.Focus()
		return
	}
	if a.win == nil {
		return
	}
	var w *mygo.Window
	w = mygo.NewWindow(mygo.WindowOptions{Title: "Settings", Width: settingsWinWidth, Height: settingsWinHeight, MinWidth: settingsWinMinWidth, MinHeight: settingsWinMinHeight, Parent: a.win, Content: ui.View(func(c *ui.Context) {
		a.applyTheme(c)
		if (c.Shortcut(0, ui.KeyEscape) || primaryShortcut(c, ui.KeyW)) && w != nil {
			w.Close()
			return
		}
		ui.Column(c).Fill().Background(c.Theme().Surface).Children(func() { a.settingsView(c) })
	})})
	centerWindowOver(w, a.win)
	a.settingsWindow = w
	w.OnFocus(func() { a.bringMainBehind(w) })
	w.OnClosed(func() { a.settingsWindow = nil })
}
