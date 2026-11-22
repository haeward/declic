package main

import (
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
)

func primaryShortcut(c *ui.Context, key ui.Key) bool {
	return c.Shortcut(ui.Cmd, key) || c.Shortcut(ui.Ctrl, key)
}

func primaryShiftShortcut(c *ui.Context, key ui.Key) bool {
	return c.Shortcut(ui.Cmd|ui.Shift, key) || c.Shortcut(ui.Ctrl|ui.Shift, key)
}

func shortcutBadge(key string) string {
	if runtime.GOOS == "darwin" {
		return "⌘" + key
	}
	return "Ctrl+" + key
}

func shortcutDisplay(key string) string {
	if runtime.GOOS == "darwin" {
		return "⌘ " + key
	}
	return "Ctrl + " + key
}

func shiftShortcutDisplay(key string) string {
	if runtime.GOOS == "darwin" {
		return "⌘ ⇧ " + key
	}
	return "Ctrl + Shift + " + key
}

var languageNames = map[string]string{
	"cpp":        "C++",
	"java":       "Java",
	"python3":    "Python3",
	"python":     "Python",
	"javascript": "JavaScript",
	"typescript": "TypeScript",
	"csharp":     "C#",
	"c":          "C",
	"golang":     "Go",
	"kotlin":     "Kotlin",
	"swift":      "Swift",
	"rust":       "Rust",
	"ruby":       "Ruby",
	"php":        "PHP",
	"dart":       "Dart",
	"scala":      "Scala",
	"elixir":     "Elixir",
	"erlang":     "Erlang",
	"racket":     "Racket",
	"mysql":      "MySQL",
	"mssql":      "MS SQL Server",
	"oraclesql":  "Oracle",
	"postgresql": "PostgreSQL",
	"pythondata": "Pandas",
	"bash":       "Bash",
}

func languageDisplayName(slug string) string {
	if name := languageNames[slug]; name != "" {
		return name
	}
	return slug
}

func editorLanguage(slug string) string {
	switch slug {
	case "python3", "python", "pythondata":
		return "Python"
	case "mysql", "mssql", "oraclesql", "postgresql":
		return "SQL"
	case "bash":
		return "Shell"
	}
	return languageDisplayName(slug)
}

func siteToday(site leetcode.Site) string {
	now := time.Now().UTC()
	if site == leetcode.CN {
		now = now.In(time.FixedZone("CST", 8*60*60))
	}
	return now.Format("2006-01-02")
}

func (a *app) toggleSidebar() {
	a.sidebarCollapsed = !a.sidebarCollapsed
}

func (a *app) ensureNavInit() {
	if len(a.navHistory) == 0 {
		screen := a.screen
		if screen == "" {
			screen = "problems"
		}
		entry := navEntry{screen: screen}
		if screen == "workspace" {
			entry.slug = a.problem.Slug
		}
		a.navHistory = []navEntry{entry}
		a.navIndex = 0
	}
}

func (a *app) recordNav(screen, slug string) {
	if a.navReplaying {
		a.navReplaying = false
		return
	}
	a.ensureNavInit()
	entry := navEntry{screen: screen}
	if screen == "workspace" {
		entry.slug = slug
	}
	if a.navIndex >= 0 && a.navIndex < len(a.navHistory) && a.navHistory[a.navIndex] == entry {
		return
	}
	a.navHistory = append(a.navHistory[:a.navIndex+1], entry)
	a.navIndex = len(a.navHistory) - 1
}

func (a *app) canGoBack() bool {
	a.ensureNavInit()
	return !a.busy && a.navIndex > 0
}

func (a *app) canGoForward() bool {
	a.ensureNavInit()
	return !a.busy && a.navIndex+1 < len(a.navHistory)
}

func (a *app) goBack() {
	if !a.canGoBack() {
		return
	}
	a.navIndex--
	a.applyNavEntry(a.navHistory[a.navIndex])
}

func (a *app) goForward() {
	if !a.canGoForward() {
		return
	}
	a.navIndex++
	a.applyNavEntry(a.navHistory[a.navIndex])
}

func (a *app) applyNavEntry(entry navEntry) {
	a.navReplaying = true
	if entry.screen == "workspace" && entry.slug != "" {
		if a.problem.Slug == entry.slug && a.problem.ID != "" {
			a.navigateNoRecord("workspace")
			a.navReplaying = false
			return
		}
		a.open(entry.slug)
		return
	}
	a.navigateNoRecord(entry.screen)
	a.navReplaying = false
}

func (a *app) view(c *ui.Context) {
	a.ensureNavInit()
	a.code.ReadOnly = a.problem.ID == "" || (a.busy && !a.saving)
	a.code.NoScrollPastEnd = true
	if v := a.code.Buffer.Version(); v != a.version {
		a.version, a.changed = v, time.Now()
	}
	a.applyTheme(c)
	t := c.Theme()
	ui.Column(c).Fill().Background(sidebarColor(c)).Children(func() {
		// Unified top chrome bar aligning macOS traffic lights with Toggle sidebar, Go back, Go forward.
		ui.Row(c).FillWidth().Height(max(44, c.TitleBar().Height)).Shrink(0).AlignItems(ui.Center).DragWindow().Children(func() {
			lead := c.TitleBar().Left
			if lead == 0 {
				lead = 14
			} else {
				lead += 20
			}
			ui.Box(c).Width(lead)
			a.windowNavControls(c)
		})
		ui.Row(c).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c)
			ui.Column(c.Key("main-content")).Grow(1).MinWidth(0).FillHeight().Background(t.Surface).Radius(12, 0, 0, 0).Clip().Children(func() {
				if a.errorStatus {
					a.statusView(c)
				}
				switch a.screen {
				case "workspace":
					a.workspaceView(c)
				default:
					a.problemsView(c)
				}
			})
		})
	})
	if primaryShortcut(c, ui.KeyB) {
		a.toggleSidebar()
	}
	if primaryShortcut(c, ui.KeyBracketLeft) || c.Shortcut(ui.Alt, ui.KeyLeft) {
		a.goBack()
	}
	if primaryShortcut(c, ui.KeyBracketRight) || c.Shortcut(ui.Alt, ui.KeyRight) {
		a.goForward()
	}
	if primaryShortcut(c, ui.KeyS) {
		a.save(nil)
	}
	if primaryShortcut(c, ui.KeyComma) {
		a.openSettings("")
	}
	if primaryShortcut(c, ui.KeyP) || primaryShortcut(c, ui.KeyK) {
		a.openQuickOpen()
	}
	if a.screen == "workspace" && !a.showQuickOpen {
		unmodifiedCode := a.isAtTemplate() || strings.TrimSpace(a.code.Buffer.String()) == ""
		if primaryShiftShortcut(c, ui.KeyEnter) {
			if !a.busy && !unmodifiedCode && !(a.judge != nil && a.judge.pending) && !time.Now().Before(a.submitAfter) {
				a.requestJudge(true)
			}
		} else if primaryShortcut(c, ui.KeyEnter) {
			if !a.busy && !unmodifiedCode && !(a.judge != nil && a.judge.pending) && !(a.username != "" && len(a.testcases) == 0) {
				a.requestJudge(false)
			}
		}
	}
	if a.screen != "workspace" && !a.showQuickOpen && !a.filterOpen && !a.sortOpen && !a.sitePopup && !a.accountPopup && !a.premiumModal && len(a.page.Problems) > 0 {
		moved := false
		if c.Shortcut(0, ui.KeyDown) {
			if a.selectedRow < 0 {
				if a.hoverRowPlus1 > 0 && a.hoverRowPlus1 <= len(a.page.Problems) {
					a.selectedRow = min(a.hoverRowPlus1, len(a.page.Problems)-1)
				} else {
					a.selectedRow = 0
				}
			} else if a.selectedRow+1 < len(a.page.Problems) {
				a.selectedRow++
			}
			a.listKeyboardNav = true
			moved = true
		}
		if c.Shortcut(0, ui.KeyUp) {
			if a.selectedRow < 0 {
				if a.hoverRowPlus1 > 1 && a.hoverRowPlus1 <= len(a.page.Problems) {
					a.selectedRow = a.hoverRowPlus1 - 2
				} else {
					a.selectedRow = len(a.page.Problems) - 1
				}
			} else if a.selectedRow > 0 {
				a.selectedRow--
			}
			a.listKeyboardNav = true
			moved = true
		}
		if moved && a.selectedRow >= 0 {
			rowH := float32(48)
			if a.compact {
				rowH = 38
			}
			viewH := a.catalogViewportH
			if viewH <= 0 {
				_, winH := c.Size()
				viewH = max(rowH*2, winH-340)
			}
			const topPad = float32(6)
			top := topPad + float32(a.selectedRow)*rowH
			bottom := top + rowH - 2
			if top < a.catalogScroll.Y {
				a.catalogScroll.Y = max(0, top-topPad)
			} else if bottom > a.catalogScroll.Y+viewH {
				a.catalogScroll.Y = bottom - viewH
			}
		}
		if c.Shortcut(0, ui.KeyEnter) && a.selectedRow >= 0 && a.selectedRow < len(a.page.Problems) && !a.busy {
			slug := a.page.Problems[a.selectedRow].Slug
			a.selectedRow = -1
			a.listKeyboardNav = false
			a.open(slug)
		}
	}
	if a.restoreTemplate && ui.AlertDialog(c, &a.restoreTemplate, "Restore code template?", "Replace the current code with the official starter template? Your replacement will be saved locally.", "Cancel", "Restore template") == 1 {
		a.restoreCodeTemplate()
	}
	if a.screen == "workspace" && primaryShortcut(c, ui.KeyJ) {
		a.testPanelCollapsed = !a.testPanelCollapsed
	}
	if a.screen == "workspace" && primaryShortcut(c, ui.KeyBackslash) {
		a.editorMaximized = !a.editorMaximized
	}
	if a.showQuickOpen {
		a.quickOpenDialog(c)
	}
	if a.premiumModal {
		a.premiumDialog(c)
	}
}

func (a *app) navigate(screen string) {
	a.ensureNavInit()
	finish := func() {
		a.screen = screen
		if screen == "workspace" {
			a.sitePopup = false
			a.recordNav("workspace", a.problem.Slug)
		} else {
			a.recordNav(screen, "")
		}
	}
	if a.screen == "workspace" && screen != "workspace" {
		a.save(finish)
		return
	}
	finish()
}

func (a *app) navigateNoRecord(screen string) {
	finish := func() {
		a.screen = screen
	}
	if a.screen == "workspace" && screen != "workspace" {
		a.save(finish)
		return
	}
	finish()
}

func (a *app) restoreCodeTemplate() {
	if a.busy || a.problem.ID == "" {
		return
	}
	for _, s := range a.problem.Snippets {
		if s.Language == a.language {
			a.code.Buffer.Set(s.Code)
			a.changed = time.Now()
			a.saveFailed = false
			return
		}
	}
}

func (a *app) renderDescription() {
	if a.preview == nil {
		return
	}
	body := a.problem.Content
	if !a.english && a.problem.TranslatedContent != "" {
		body = a.problem.TranslatedContent
	}
	a.preview.Page().LoadHTML(a.statementHTML(body), "")
}

func displayDifficulty(d string) string {
	if d == "" {
		return "—"
	}
	return d
}
