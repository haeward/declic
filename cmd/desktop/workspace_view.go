package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/haeward/declic/internal/editorutil"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/workspace"
)

func (a *app) problemURL() string {
	return a.site.URL() + "/problems/" + url.PathEscape(a.problem.Slug) + "/"
}

func (a *app) workspaceView(c *ui.Context) {
	w, _ := c.Size()
	ui.Box(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		a.renderPrewarmedVideos(c)
		ui.Column(c).Fill().Background(c.Theme().Surface).Children(func() {
			if a.editorMaximized {
				a.editorView(c)
				return
			}
			if w < 1050 {
				ui.Row(c).FillWidth().Height(44).Shrink(0).Padding(0, 16).Gap(20).BorderWidth(0, 0, 1, 0).BorderColor(hairlineColor(c)).Children(func() {
					tabButton(c, "Description", a.narrowPane == "description").OnClick(func() { a.narrowPane = "description" })
					tabButton(c, "Editor", a.narrowPane != "description").OnClick(func() { a.narrowPane = "editor" })
				})
				if a.narrowPane == "description" {
					a.descriptionView(c, false)
				} else {
					a.editorView(c)
				}
			} else {
				a.split = max(38, min(58, a.split))
				ui.SplitPercent(c, &a.split, func() { a.descriptionView(c, true) }, func() { a.editorView(c) }).Grow(1).FillWidth()
			}
		})
	})
}

func (a *app) descriptionView(c *ui.Context, rightBorder bool) {
	t := c.Theme()
	col := ui.Column(c.Key("resources")).Fill().MinWidth(0).Background(t.Surface).Clip()
	if rightBorder {
		col.BorderWidth(0, 1, 0, 0).BorderColor(hairlineColor(c))
	}
	col.Children(func() {
		if a.site == leetcode.CN && a.resourceTab == "Editorial" {
			a.resourceTab = "Description"
		}
		ui.Row(c).FillWidth().Height(44).Shrink(0).Padding(0, 22).Gap(18).BorderWidth(0, 0, 1, 0).BorderColor(hairlineColor(c)).Children(func() {
			type tabSpec struct {
				name string
				icon *ui.SVG
				tint ui.Color
			}
			tabs := []tabSpec{{"Description", iconDescription, t.Accent}}
			if a.site != leetcode.CN {
				tabs = append(tabs, tabSpec{"Editorial", iconEditorial, ui.Hex("#d97706")})
			}
			tabs = append(tabs,
				tabSpec{"Solutions", iconSolutions, ui.Hex("#0284c7")},
				tabSpec{"Submissions", iconSubmissions, t.Accent},
			)
			for _, tab := range tabs {
				selected := a.resourceTab == tab.name || (a.resourceTab == "" && tab.name == "Description")
				name := tab.name
				tabButtonWithIcon(c, tab.icon, name, selected, tab.tint).OnClick(func() {
					if a.resourceTab != name {
						a.pauseInlineVideos()
						a.resourceTab = name
					}
				})
			}
		})
		if a.resourceTab != "" && a.resourceTab != "Description" {
			a.resourceView(c)
			return
		}
		a.problemContextHeader(c, true)
		if a.preview != nil {
			ui.WebView(c, a.preview).Fill()
		}
	})
}

func (a *app) problemContextHeader(c *ui.Context, showLangToggle bool) {
	t := c.Theme()
	title := strings.TrimSpace(a.problem.FrontendID) + ". " + strings.TrimSpace(a.problem.Title)
	ui.Row(c).FillWidth().Padding(16, 22, 8, 22).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(22).Bold().Shrink(1).MinWidth(0)
		openBtn := ui.ButtonBase(c).Label("Open problem on LeetCode").Size(30, 30).Radius(7).Center().Shrink(0)
		openFg := t.TextMuted
		if openBtn.Hovered() {
			openBtn.Background(t.SurfaceHover)
			openFg = t.Text
		}
		openBtn.Children(func() {
			ui.Icon(c, iconExternal).FontSize(16.5).TextColor(openFg)
		}).OnClick(func() { mygo.Shell.OpenExternal(a.problemURL()) })
		attachTooltip(c, openBtn, "Open on "+string(a.site), tipBottom)
	})
	ui.Row(c).FillWidth().Shrink(0).Padding(0, 22, 10, 22).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
			a.problemBadges(c, a.problem)
		})
		if showLangToggle && a.site == leetcode.CN && a.problem.Content != "" && a.problem.TranslatedContent != "" {
			quietButton(c, "中文 / EN").OnClick(func() { a.english = !a.english; a.renderDescription() })
		}
	})
}

func (a *app) editorView(c *ui.Context) {
	ui.Column(c.Key("editor-pane")).Fill().MinWidth(0).Background(c.Theme().Surface).Children(func() {
		if a.testPanelCollapsed {
			a.codeCard(c)
			a.testPanel(c)
			return
		}
		if a.editorSplit == 0 {
			a.editorSplit = 68
		}
		a.editorSplit = max(35, min(80, a.editorSplit))
		ui.SplitVerticalPercent(c, &a.editorSplit, func() { a.codeCard(c) }, func() { a.testPanel(c) }).Fill()
	})
}

func (a *app) isAtTemplate() bool {
	if a.problem.ID == "" {
		return true
	}
	cur := a.code.Buffer.String()
	for _, s := range a.problem.Snippets {
		if s.Language == a.language {
			return cur == s.Code
		}
	}
	return false
}

func effectiveTabSize(lang string, pref int) int {
	if pref == 2 || pref == 4 {
		return pref
	}
	switch lang {
	case "javascript", "typescript", "ruby", "dart", "elixir", "scala", "racket":
		return 2
	default:
		return 4
	}
}

func (a *app) codeCard(c *ui.Context) {
	t := c.Theme()
	a.code.Theme = &editor.Theme{Background: t.Surface}
	a.code.NoScrollPastEnd = true
	if a.editorFontSize >= 11 && a.editorFontSize <= 18 {
		a.code.Font = ui.Font{Size: float32(a.editorFontSize)}
	}
	if a.editorTabSize == 2 || a.editorTabSize == 4 {
		a.code.TabSize = a.editorTabSize
		a.code.Indent = editor.IndentSpaces
	} else {
		a.code.TabSize = 0
		a.code.Indent = editor.IndentAuto
	}
	ui.Column(c).FillWidth().Grow(1).MinHeight(0).Background(t.Surface).Clip().Children(func() {
		now := time.Now()
		src := a.code.Buffer.String()
		srcEmpty := strings.TrimSpace(src) == ""
		atTemplate := a.isAtTemplate()
		ui.Row(c).FillWidth().Height(44).Shrink(0).Padding(0, 14, 0, 16).Gap(4).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(hairlineColor(c)).Children(func() {
			a.languagePicker(c)
			ui.Box(c).Grow(1)
			fmtIcon, fmtFg, fmtTip := iconFormat, t.TextMuted, "Format code"
			if a.code.ReadOnly || srcEmpty {
				fmtFg = t.TextMuted.Alpha(.38)
			} else if now.Before(a.codeFormattedUntil) {
				fmtIcon, fmtFg, fmtTip = iconCheck, t.Success, "Formatted"
			}
			iconActionButtonTip(c, "Format code", fmtTip, fmtIcon, fmtFg, t.SurfaceHover, tipBottom).Disabled(a.code.ReadOnly || srcEmpty).OnClick(func() {
				formatted := editorutil.FormatCode(a.language, a.code.Buffer.String(), effectiveTabSize(a.language, a.editorTabSize))
				if formatted != a.code.Buffer.String() {
					a.code.Buffer.Set(formatted)
					a.changed = time.Now()
					a.saveFailed = false
				}
				a.codeFormattedUntil = time.Now().Add(1500 * time.Millisecond)
			})
			copyIcon, copyFg, copyTip := iconDuplicate, t.TextMuted, "Copy code"
			if src == "" {
				copyFg = t.TextMuted.Alpha(.38)
			} else if now.Before(a.codeCopiedUntil) {
				copyIcon, copyFg, copyTip = iconCheck, t.Success, "Copied"
			}
			iconActionButtonTip(c, "Copy code", copyTip, copyIcon, copyFg, t.SurfaceHover, tipBottom).Disabled(src == "").OnClick(func() {
				c.WriteClipboard(a.code.Buffer.String())
				a.codeCopiedUntil = time.Now().Add(1500 * time.Millisecond)
			})
			retDisabled := a.username == "" || a.busy || a.retrievingSub || a.problem.Slug == ""
			retIcon, retFg, retTip := iconRetrieve, t.TextMuted, "Retrieve last submitted code"
			if a.username == "" {
				retFg = t.TextMuted.Alpha(.38)
				retTip = "Sign in to retrieve last submitted code"
			} else if a.retrievingSub {
				retFg = accentLinkColor(c)
				retTip = "Retrieving last submitted code…"
			} else if now.Before(a.codeRetrievedUntil) {
				retIcon, retFg, retTip = iconCheck, t.Success, "Retrieved last submitted code"
			}
			iconActionButtonTip(c, "Retrieve last submitted code", retTip, retIcon, retFg, t.SurfaceHover, tipBottom).Disabled(retDisabled).OnClick(a.retrieveLastSubmittedCode)
			resetDisabled := a.busy || atTemplate
			resetFg := t.TextMuted
			if resetDisabled {
				resetFg = t.TextMuted.Alpha(.38)
			}
			iconActionButtonTip(c, "Reset template", "Reset template", iconReset, resetFg, t.SurfaceHover, tipBottom).Disabled(resetDisabled).OnClick(func() { a.restoreTemplate = true })
			ui.Box(c).Width(1).Height(16).Background(hairlineColor(c)).Margin(0, 3)
			a.editorQuickSettings(c)
			focusIcon, focusLabel, focusTip := iconMaximize, "Focus editor", "Focus editor ("+shortcutBadge("\\")+")"
			focusFg := t.TextMuted
			if a.editorMaximized {
				focusIcon, focusLabel, focusTip = iconMinimize, "Show problem", "Show problem ("+shortcutBadge("\\")+")"
				focusFg = accentLinkColor(c)
			}
			iconActionButtonTip(c, focusLabel, focusTip, focusIcon, focusFg, t.SurfaceHover, tipBottom).OnClick(func() { a.editorMaximized = !a.editorMaximized })
		})
		code := editor.View(c.Key("workspace-code"), a.code).Grow(1).FillWidth().MinHeight(120).Disabled(a.code.ReadOnly)
		if a.focusCode {
			code.Focus()
			a.focusCode = false
		}
		ui.Row(c).FillWidth().Height(54).Shrink(0).Padding(0, 16).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(hairlineColor(c)).Children(func() {
			pos := a.code.CursorPos()
			ui.Text(c, fmt.Sprintf("Ln %d, Col %d", pos.Line+1, pos.Col+1)).FontSize(12).TextColor(t.TextMuted)
			if a.saveFailed {
				linkButton(c, "Retry save").Disabled(a.busy).OnClick(func() { a.save(nil) })
			}
			ui.Box(c).Grow(1)
			unmodifiedCode := srcEmpty || atTemplate
			runDisabled := a.busy || unmodifiedCode || a.judge != nil && a.judge.pending || a.username != "" && len(a.testcases) == 0
			submitDisabled := a.busy || unmodifiedCode || a.judge != nil && a.judge.pending || time.Now().Before(a.submitAfter)
			outlineButton(c, "Run", iconPlay, 34).Disabled(runDisabled).OnClick(func() { a.requestJudge(false) })
			primaryActionButton(c, "Submit", iconUpload, 34).Disabled(submitDisabled).OnClick(func() { a.requestJudge(true) })
		})
	})
}

func (a *app) languagePicker(c *ui.Context) {
	t := c.Theme()
	label := languageDisplayName(a.language)
	anchor := ui.ButtonBase(c).Label("Language: "+label).MinWidth(138).Height(30).Padding(0, 12).Radius(8).Background(chipBg(c)).Disabled(a.busy)
	if anchor.Hovered() {
		anchor.Background(t.SurfacePressed)
	}
	applyPressFeedback(anchor, 0.975)
	anchor.Children(func() {
		ui.Row(c).Fill().Gap(10).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconCode).FontSize(14).TextColor(t.TextMuted)
				ui.Text(c, label).FontSize(12.5).FontWeight(500).TextColor(t.Text).SingleLine()
			})
			ui.Icon(c, iconChevronDown).FontSize(13).TextColor(t.TextMuted)
		})
	}).OnClick(func() {
		a.langOpen = !a.langOpen
		if a.langOpen {
			a.editorSettingsOpen = false
		}
	})

	if !a.langOpen {
		return
	}
	var supported []leetcode.Snippet
	for _, s := range a.problem.Snippets {
		if workspace.Extension(s.Language) != "" {
			supported = append(supported, s)
		}
	}
	ui.PopoverBase(c, anchor, &a.langOpen, func(panel ui.Element) {
		panel.Padding(10).Radius(12).Background(t.Surface).Border(1, t.Border.Alpha(.55)).Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, .20))
		if len(supported) == 0 {
			ui.Text(c, "No languages available").FontSize(12.5).TextColor(t.TextMuted).Padding(8, 12)
			return
		}
		perCol := 8
		if len(supported) <= 6 {
			perCol = len(supported)
		}
		cols := (len(supported) + perCol - 1) / perCol
		if cols > 3 {
			cols = 3
			perCol = (len(supported) + cols - 1) / cols
		}
		ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
			for col := 0; col < cols; col++ {
				start := col * perCol
				end := min(len(supported), start+perCol)
				if start >= end {
					break
				}
				if col > 0 {
					ui.Box(c).Width(1).FillHeight().Background(hairlineColor(c)).Margin(4, 2)
				}
				ui.Column(c).Width(148).Gap(2).Children(func() {
					for _, s := range supported[start:end] {
						name := languageDisplayName(s.Language)
						selected := a.language == s.Language
						b := ui.ButtonBase(c).Label(name).FillWidth().Height(32).Padding(0, 10).Radius(7).Disabled(a.busy)
						if selected {
							b.Background(activeChipBg(c))
						} else if b.Hovered() {
							b.Background(t.SurfaceHover)
						}
						b.Children(func() {
							ui.Row(c).Fill().Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Box(c).Width(14).Center().Children(func() {
									if selected {
										ui.Icon(c, iconCheck).FontSize(13).TextColor(accentLinkColor(c))
									}
								})
								fg := t.Text
								weight := 400
								if selected {
									fg = accentLinkColor(c)
									weight = 600
								}
								ui.Text(c, name).FontSize(13).FontWeight(weight).TextColor(fg).SingleLine()
							})
						}).OnClick(func() {
							a.langOpen = false
							if s.Language != a.language {
								a.save(func() { a.load(a.problem, s.Language) })
							}
						})
					}
				})
			}
		})
	})
}

func (a *app) editorQuickSettings(c *ui.Context) {
	t := c.Theme()
	btn := ui.ButtonBase(c).Label("Editor settings").Size(28, 28).Radius(7).Center()
	fg := t.TextMuted
	if a.editorSettingsOpen {
		btn.Background(activeChipBg(c))
		fg = accentLinkColor(c)
	} else if btn.Hovered() {
		btn.Background(t.SurfaceHover)
		fg = t.Text
	}
	applyPressFeedback(btn, 0.94)
	btn.Children(func() { ui.Icon(c, iconSettings).FontSize(15).TextColor(fg) }).OnClick(func() {
		a.editorSettingsOpen = !a.editorSettingsOpen
		if a.editorSettingsOpen {
			a.langOpen = false
		}
	})
	attachTooltip(c, btn, "Editor settings", tipBottom)
	if !a.editorSettingsOpen {
		return
	}
	ui.PopoverBase(c, btn, &a.editorSettingsOpen, func(panel ui.Element) {
		panel.AttachTo(btn, ui.AnchorBottomEnd, ui.AnchorTopEnd).Margin(6, 0, 0, 0)
		panel.Width(276).Padding(14).Radius(12).Background(t.Surface).Border(1, t.Border.Alpha(.55)).Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, .20))
		ui.Column(c).FillWidth().Gap(10).Children(func() {
			ui.Text(c, "Editor settings").FontSize(12).FontWeight(600).TextColor(t.TextMuted)
			quickSettingRow(c, "Font size", func() {
				size := a.editorFontSize
				if size < 11 || size > 18 {
					size = 13
				}
				segmentedTrack(c, func() {
					segmentItem(c, "−", false, t.TextMuted, 26).Label("Decrease quick font size").Disabled(size <= 11).OnClick(func() {
						a.editorFontSize = max(11, size-1)
					})
					segmentItem(c, fmt.Sprintf("%d pt", size), true, t.Text, 26)
					segmentItem(c, "+", false, t.TextMuted, 26).Label("Increase quick font size").Disabled(size >= 18).OnClick(func() {
						a.editorFontSize = min(18, size+1)
					})
				})
			})
			quickSettingRow(c, "Tab width", func() {
				tab := a.editorTabSize
				if tab != 0 && tab != 2 && tab != 4 {
					tab = 0
				}
				segmentedTrack(c, func() {
					for _, opt := range []struct {
						text, label string
						val         int
					}{{"Auto", "Auto indent", 0}, {"2", "2 spaces", 2}, {"4", "4 spaces", 4}} {
						active := tab == opt.val
						itemFg := t.TextMuted
						if active {
							itemFg = t.Text
						}
						v := opt.val
						segmentItem(c, opt.text, active, itemFg, 26).Label(opt.label).OnClick(func() {
							a.editorTabSize = v
						})
					}
				})
			})
			quickSettingRow(c, "Word wrap", func() {
				segmentedTrack(c, func() {
					for _, opt := range []struct {
						label string
						on    bool
					}{{"On", true}, {"Off", false}} {
						active := a.code.SoftWrap == opt.on
						itemFg := t.TextMuted
						if active {
							itemFg = t.Text
						}
						segmentItem(c, opt.label, active, itemFg, 26).Label("Word wrap " + opt.label).OnClick(func() {
							a.code.SoftWrap = opt.on
						})
					}
				})
			})
			quickSettingRow(c, "Auto-close", func() {
				segmentedTrack(c, func() {
					for _, opt := range []struct {
						label string
						on    bool
					}{{"On", true}, {"Off", false}} {
						active := !a.code.NoAutoClose == opt.on
						itemFg := t.TextMuted
						if active {
							itemFg = t.Text
						}
						segmentItem(c, opt.label, active, itemFg, 26).Label("Auto-close " + opt.label).OnClick(func() {
							a.code.NoAutoClose = !opt.on
						})
					}
				})
			})
		})
	})
}

func quickSettingRow(c *ui.Context, label string, control func()) {
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12.5).TextColor(c.Theme().Text).Grow(1).SingleLine()
		control()
	})
}

func (a *app) testcasesEdited() bool {
	if len(a.testcases) != len(a.problem.Examples) {
		return true
	}
	for i := range a.testcases {
		if a.testcases[i] != a.problem.Examples[i] {
			return true
		}
	}
	return false
}

func (a *app) testPanel(c *ui.Context) {
	t := c.Theme()
	panel := ui.Column(c).FillWidth().Shrink(0).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(hairlineColor(c)).Clip()
	if !a.testPanelCollapsed {
		panel.FillHeight()
	}
	panel.Children(func() {
		ui.Row(c).FillWidth().Height(44).Shrink(0).Padding(0, 12).Gap(4).AlignItems(ui.Center).Children(func() {
			pillTabButton(c, "Testcase", !a.testPanelCollapsed && a.resultTab != "result").OnClick(func() { a.resultTab = "testcase"; a.testPanelCollapsed = false })
			pillTabButton(c, "Test Result", !a.testPanelCollapsed && a.resultTab == "result").OnClick(func() { a.resultTab = "result"; a.testPanelCollapsed = false })
			ui.Box(c).Grow(1)
			if a.testPanelCollapsed {
				ui.Text(c, fmt.Sprintf("%d cases", len(a.testcases))).FontSize(12).TextColor(t.TextMuted).Padding(0, 4)
			}
			label, icon := "Collapse", iconChevronDown
			if a.testPanelCollapsed {
				label, icon = "Expand", iconChevronUp
			}
			iconActionButtonTip(c, label, label, icon, t.TextMuted, t.SurfaceHover, tipTop).OnClick(func() { a.testPanelCollapsed = !a.testPanelCollapsed })
		})
		if !a.testPanelCollapsed {
			ui.Column(c).FillWidth().Grow(1).MinHeight(0).Padding(4, 16, 16, 16).Gap(10).Children(func() {
				if a.resultTab == "result" {
					a.judgeResult(c)
				} else {
					a.testcaseEditor(c)
				}
			})
		}
	})
}

func (a *app) retrieveLastSubmittedCode() {
	if a.busy || a.retrievingSub || a.problem.Slug == "" {
		return
	}
	if a.username == "" {
		a.openLogin()
		a.accountNotice = "Sign in to retrieve your last submitted code."
		return
	}
	a.retrievingSub = true
	site, user, slug, curLang := a.site, a.username, a.problem.Slug, a.language
	supported := make(map[string]bool, len(a.problem.Snippets))
	for _, sn := range a.problem.Snippets {
		if workspace.Extension(sn.Language) != "" {
			supported[sn.Language] = true
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		client := leetcode.New(site)
		cookies, _, err := verifiedCookiesCached(ctx, client, user)
		var picked *leetcode.Submission
		var code string
		if err == nil {
			var history []leetcode.Submission
			history, _, err = client.Submissions(ctx, slug, 0, cookies)
			if err == nil {
				for i := range history {
					if history[i].Language == curLang {
						sub := history[i]
						picked = &sub
						break
					}
				}
				if picked == nil {
					for i := range history {
						if supported[history[i].Language] {
							sub := history[i]
							picked = &sub
							break
						}
					}
				}
				if picked != nil {
					var id int
					id, err = strconv.Atoi(picked.ID)
					if err == nil {
						code, err = client.SubmissionCode(ctx, id, cookies)
					}
				}
			}
		}
		a.post(func() {
			a.retrievingSub = false
			if a.closed || a.site != site || a.username != user || a.problem.Slug != slug {
				return
			}
			if err != nil {
				a.status = err.Error()
				a.errorStatus = true
				if errors.Is(err, errSessionExpired) {
					a.expireSession()
				}
				return
			}
			if picked == nil || code == "" {
				a.status = "No previous submission found for this problem."
				a.errorStatus = true
				return
			}
			picked.Code = code
			if a.useSubmission(*picked) {
				a.changed = time.Now()
				a.saveFailed = false
				a.codeRetrievedUntil = time.Now().Add(1500 * time.Millisecond)
			}
		})
	}()
}

func (a *app) testcaseEditor(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Wrap().Padding(1, 4, 0, 0).Gap(4).AlignItems(ui.Center).Children(func() {
		canDelete := len(a.testcases) > 1
		for i := range a.testcases {
			idx := i
			label := fmt.Sprintf("Case %d", idx+1)
			if idx < len(a.problem.Examples) && a.problem.Examples[idx] != a.testcases[idx] {
				label += " •"
			}
			active := idx == a.caseIndex
			wrap := ui.Box(c.Key(fmt.Sprintf("case-chip-%d", idx))).Padding(5, 5, 0, 0)
			showDelete := canDelete && (wrap.Hovered() || (a.caseChipSelected && active))
			wrap.Children(func() {
				chipButton(c, label, active).OnClick(func() {
					a.caseIndex = idx
					a.caseChipSelected = true
				})
				if showDelete {
					delBtn := ui.ButtonBase(c).Label(fmt.Sprintf("Delete Case %d", idx+1)).Absolute().Top(0).Right(0).Size(16, 16).Radius(8).Center().Background(t.Surface).Border(1, t.Border)
					delFg := t.TextMuted
					if delBtn.Hovered() {
						delBtn.Background(t.Danger).BorderColor(t.Danger)
						delFg = t.AccentText
					}
					delBtn.Children(func() {
						ui.Icon(c, iconClose).FontSize(9).TextColor(delFg)
					}).OnClick(func() {
						a.testcases = append(a.testcases[:idx], a.testcases[idx+1:]...)
						if idx < a.caseIndex {
							a.caseIndex--
						} else if a.caseIndex >= len(a.testcases) {
							a.caseIndex = max(0, len(a.testcases)-1)
						}
						if len(a.testcases) <= 1 {
							a.caseChipSelected = false
						}
					})
				}
			})
		}
		ui.Row(c).Padding(5, 0, 0, 2).Gap(6).AlignItems(ui.Center).Children(func() {
			chipButton(c, "+", false).Label("Add case").OnClick(func() {
				a.testcases = append(a.testcases, "")
				a.caseIndex = len(a.testcases) - 1
				a.caseChipSelected = true
			})
			if len(a.problem.Examples) > 0 {
				resetFg := t.TextMuted
				if a.testcasesEdited() {
					resetFg = accentLinkColor(c)
				}
				iconActionButtonTip(c, "Reset", "Reset testcases", iconReset, resetFg, t.SurfaceHover, tipTop).OnClick(func() {
					a.testcases = append([]string(nil), a.problem.Examples...)
					a.caseIndex = 0
					a.caseChipSelected = false
				})
			}
		})
	})
	if len(a.testcases) == 0 {
		ui.Row(c).FillWidth().Padding(8, 2).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "No testcases").FontSize(13).TextColor(t.TextMuted)
			if len(a.problem.Examples) > 0 {
				outlineButton(c, "Reset", iconReset, 30).OnClick(func() {
					a.testcases = append([]string(nil), a.problem.Examples...)
					a.caseIndex = 0
				})
			}
		})
		return
	}
	var metadata struct{ Params []struct{ Name string } }
	_ = json.Unmarshal([]byte(a.problem.Metadata), &metadata)
	if len(metadata.Params) == 0 {
		ui.TextArea(c.Key("test-input"), &a.testcases[a.caseIndex]).Label("Input").FillWidth().Grow(1).Background(insetBg(c)).Radius(8)
	} else {
		values := strings.Split(a.testcases[a.caseIndex], "\n")
		for len(values) < len(metadata.Params) {
			values = append(values, "")
		}
		ui.Scroll(c).Scrollbars(false).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				for i, param := range metadata.Params {
					ui.Column(c).FillWidth().Gap(4).Children(func() {
						ui.Text(c, param.Name+" =").FontSize(12).TextColor(t.TextMuted)
						ui.TextInput(c.Key(fmt.Sprintf("case-%d-%d", a.caseIndex, i)), &values[i]).Label(param.Name).FillWidth().Radius(8).Background(insetBg(c)).Border(0, ui.Transparent).Padding(7, 10)
					})
				}
			})
		})
		a.testcases[a.caseIndex] = strings.Join(values, "\n")
	}
}
