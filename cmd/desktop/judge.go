package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/haeward/declic/internal/leetcode"
)

type judgeState struct {
	cancel          context.CancelFunc
	pending, submit bool
	id, notice      string
	stage           string
	language        string
	result          leetcode.Judgment
	inputs          []string
	selected        int
	expanded        map[string]bool
}

func (a *app) requestJudge(submit bool) {
	if a.username == "" {
		a.openLogin()
		a.accountNotice = "Sign in to use the online judge. Your current draft is saved before changing accounts."
		return
	}
	if a.judge != nil && a.judge.pending || submit && time.Now().Before(a.submitAfter) {
		return
	}
	a.testPanelCollapsed = false
	a.resultTab = "result"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	state := &judgeState{pending: true, submit: submit, cancel: cancel, stage: "Verifying session…", language: a.language, inputs: append([]string(nil), a.testcases...)}
	a.judge = state
	site, user, p, lang, code := a.site, a.username, a.problem, a.language, a.code.Buffer.String()
	setStage := func(msg string) {
		a.post(func() {
			if !a.closed && a.judge == state && state.pending {
				state.stage = msg
			}
		})
	}
	go func() {
		defer cancel()
		var result leetcode.Judgment
		var id, notice string
		sent := false
		defer func() {
			a.post(func() {
				if a.closed || a.judge != state {
					return
				}
				if notice == "Session expired. Sign in again." {
					a.expireSession()
				}
				state.result, state.id, state.notice, state.pending = result, id, notice, false
				if submit && sent && result.Verdict == "Accepted" {
					delete(a.resources, "Submissions")
					if a.solved == nil {
						a.solved = map[string]bool{}
					}
					a.solved[p.Slug] = true
					if a.catalog != nil {
						a.filterCatalog(a.search, a.skip)
					}
				}
				if submit && sent {
					a.submitAfter = time.Now().Add(15 * time.Second)
				}
			})
		}()
		client := leetcode.New(site)
		cookies, _, err := verifiedCookiesCached(ctx, client, user)
		if err != nil {
			notice = err.Error()
			if err == errSessionExpired {
				notice = "Session expired. Sign in again."
			}
			return
		}
		if ctx.Err() != nil {
			notice = "Canceled"
			return
		}
		if submit {
			setStage("Submitting code…")
		} else {
			setStage("Running testcases…")
		}
		sent = true
		id, err = client.StartJudge(ctx, submit, p, lang, code, state.inputs, cookies)
		if err != nil {
			maybeInvalidateJudgeSession(site, err)
			notice = judgeErrorNotice(submit, false, err, "")
			return
		}
		setStage("Judging…")
		for {
			select {
			case <-ctx.Done():
				err = ctx.Err()
			case <-time.After(1500 * time.Millisecond):
				result, err = client.CheckJudge(ctx, id, p.Slug, cookies)
			}
			if err != nil {
				maybeInvalidateJudgeSession(site, err)
				notice = judgeErrorNotice(submit, true, err, "")
				return
			}
			if result.State == "SUCCESS" {
				return
			}
			if result.State != "PENDING" && result.State != "STARTED" {
				notice = judgeErrorNotice(submit, true, nil, result.State)
				return
			}
		}
	}()
}

func maybeInvalidateJudgeSession(site leetcode.Site, err error) {
	var statusErr *leetcode.StatusError
	if errors.As(err, &statusErr) && (statusErr.StatusCode == http.StatusUnauthorized || statusErr.StatusCode == http.StatusForbidden) {
		invalidateSessionCache(site)
	}
}

func judgeErrorNotice(submit, polling bool, err error, state string) string {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			if submit && polling {
				return "Stopped waiting — check Submissions"
			}
			return "Canceled"
		}
		if errors.Is(err, context.DeadlineExceeded) {
			if submit {
				return "Result unknown — check Submissions"
			}
			if polling {
				return "Timed out waiting for test result"
			}
			return "Timed out running testcases"
		}
		if submit && polling {
			return err.Error() + " — check Submissions"
		}
		return err.Error()
	}
	if state != "" {
		if submit {
			return fmt.Sprintf("Judge returned %s — check Submissions", state)
		}
		return fmt.Sprintf("Judge returned %s", state)
	}
	if submit {
		return "Result unknown — check Submissions"
	}
	return "Judge returned an incomplete response"
}
func (a *app) judgeResult(c *ui.Context) {
	s := a.judge
	if s == nil {
		ui.Text(c, "No execution results").Bold()
		return
	}
	if s.pending {
		stage := s.stage
		if stage == "" {
			stage = "Judging…"
		}
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, stage).FontSize(14).Bold()
			outlineButton(c, "Stop waiting", nil, 30).OnClick(s.cancel)
		})
		return
	}
	if s.notice != "" {
		ui.Text(c, s.notice)
		if s.notice == "Session expired. Sign in again." {
			quietButton(c, "Sign in again").OnClick(a.openLogin)
		} else if s.submit {
			ui.Link(c, "View submissions ↗", a.problemURL()+"submissions/")
		}
		return
	}
	r := s.result
	t := c.Theme()
	ui.Scroll(c).Scrollbars(false).Fill().Children(func() {
		ui.Column(c).FillWidth().Gap(10).Children(func() {
			title := r.Verdict
			if !s.submit && len(r.Compare) == len(s.inputs) && r.Compare != "" {
				title = fmt.Sprintf("%d of %d cases passed", strings.Count(r.Compare, "1"), len(s.inputs))
			} else if !s.submit && r.Correct != nil && (r.Verdict == "Accepted" || r.Verdict == "Wrong Answer" || r.Verdict == "") {
				if *r.Correct {
					title = "All cases passed"
				} else {
					title = "Test failed"
				}
			}
			allCasesPassed := !s.submit && r.Correct == nil && r.Compare != "" && len(r.Compare) == len(s.inputs) && strings.Count(r.Compare, "1") == len(s.inputs)
			color := t.Danger
			if title == "Accepted" || title == "All cases passed" || allCasesPassed || !s.submit && r.Correct != nil && *r.Correct {
				color = t.Success
			}
			if s.submit {
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, title).FontSize(16).Bold().TextColor(color)
						var meta []string
						if r.Total > 0 {
							meta = append(meta, fmt.Sprintf("%d / %d testcases passed", r.Passed, r.Total))
						}
						if lang := languageDisplayName(s.language); lang != "" {
							meta = append(meta, lang)
						}
						if len(meta) > 0 {
							ui.Text(c, strings.Join(meta, " · ")).FontSize(12).TextColor(t.TextMuted)
						}
					})
					if r.LastInput != "" {
						outlineButton(c, "Add to testcases", nil, 30).OnClick(func() {
							a.testcases = append(a.testcases, r.LastInput)
							a.caseIndex = len(a.testcases) - 1
							a.resultTab = "testcase"
						})
					}
				})
				if r.Runtime != "" || r.Memory != "" {
					ui.Row(c).FillWidth().Gap(10).Children(func() {
						for _, m := range []struct {
							title, value string
							percent      *float64
						}{{"Runtime", r.Runtime, r.RuntimePercentile}, {"Memory", r.Memory, r.MemoryPercentile}} {
							if m.value != "" {
								ui.Column(c).Grow(1).Width(0).Padding(10, 12).Gap(2).Radius(10).Background(insetBg(c)).Children(func() {
									ui.Text(c, m.title).FontSize(12).TextColor(t.TextMuted)
									ui.Text(c, m.value).FontSize(18).Bold().TextColor(t.Text)
									if m.percent != nil {
										ui.Text(c, fmt.Sprintf("Beats %.1f%%", *m.percent)).FontSize(12).TextColor(t.TextMuted)
									}
								})
							}
						}
					})
				}
				if r.LastInput != "" {
					resultFieldBox(c, s, "sub-input", "Input", r.LastInput)
				}
				if r.LastOutput != "" || r.LastExpected != "" {
					resultCompareBoxes(c, s, "sub", r.LastOutput, r.LastExpected)
				}
			} else {
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, title).FontSize(16).Bold().TextColor(color)
					if r.Runtime != "" {
						ui.Text(c, "Runtime "+r.Runtime).FontSize(12.5).TextColor(t.TextMuted)
					}
				})
				ui.Row(c).FillWidth().Gap(6).Children(func() {
					for i := range s.inputs {
						label := fmt.Sprintf("Case %d", i+1)
						if i < len(r.Compare) {
							if r.Compare[i] == '1' {
								label += " ✓"
							} else {
								label += " ✗"
							}
						}
						chipButton(c, label, s.selected == i).OnClick(func() { s.selected = i })
					}
				})
				at := func(values []string) string {
					if s.selected < len(values) {
						return values[s.selected]
					}
					return ""
				}
				caseKey := fmt.Sprintf("case-%d", s.selected)
				if v := at(s.inputs); v != "" {
					resultFieldBox(c, s, caseKey+"-in", "Input", v)
				}
				out, exp := at(r.Outputs), at(r.Expected)
				if out != "" || exp != "" {
					resultCompareBoxes(c, s, caseKey, out, exp)
				}
				if v := at(r.Stdout); v != "" {
					resultFieldBox(c, s, caseKey+"-stdout", "Stdout", v)
				}
			}
			for _, msg := range []string{r.CompileError, r.RuntimeError} {
				if msg != "" {
					ui.Column(c).FillWidth().Padding(10, 12).Radius(8).Background(t.Danger.Alpha(.08)).Children(func() {
						ui.Text(c, msg).FontSize(12).TextColor(t.Danger)
					})
					ui.Row(c).Gap(8).Children(func() {
						outlineButton(c, "Copy error", nil, 28).OnClick(func() { c.WriteClipboard(msg) })
						if match := judgeErrorLine.FindStringSubmatch(msg); len(match) > 1 {
							line, _ := strconv.Atoi(match[1])
							outlineButton(c, "Line "+match[1], nil, 28).OnClick(func() {
								pos := a.code.Buffer.Offset(editor.Pos{Line: max(0, line-1)})
								a.code.Select(pos, pos)
								a.focusCode = true
							})
						}
					})
				}
			}
		})
	})
}

const (
	resultCollapseRunes = 240
	resultCollapseLines = 6
)

func diffCommonPrefixSuffix(a, b string) (prefix, midA, midB, suffix string) {
	ra, rb := []rune(a), []rune(b)
	p := 0
	for p < len(ra) && p < len(rb) && ra[p] == rb[p] {
		p++
	}
	ea, eb := len(ra), len(rb)
	for ea > p && eb > p && ra[ea-1] == rb[eb-1] {
		ea--
		eb--
	}
	return string(ra[:p]), string(ra[p:ea]), string(rb[p:eb]), string(ra[ea:])
}

func truncateResultText(s string) (preview string, truncated bool) {
	lines := strings.Split(s, "\n")
	if len(lines) > resultCollapseLines {
		s = strings.Join(lines[:resultCollapseLines-1], "\n")
		truncated = true
	}
	runes := []rune(s)
	if len(runes) > resultCollapseRunes {
		s = string(runes[:resultCollapseRunes])
		truncated = true
	}
	if truncated {
		return strings.TrimRight(s, " \t\n") + "…", true
	}
	return s, false
}

func resultCompareBoxes(c *ui.Context, s *judgeState, keyPrefix, out, exp string) {
	mismatch := out != "" && exp != "" && out != exp
	var prefix, midOut, midExp, suffix string
	if mismatch {
		prefix, midOut, midExp, suffix = diffCommonPrefixSuffix(out, exp)
	}
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		if out != "" {
			ui.Column(c).Grow(1).Width(0).Children(func() {
				if mismatch {
					resultDiffFieldBox(c, s, keyPrefix+"-out", "Output", out, prefix, midOut, suffix, c.Theme().Danger)
				} else {
					resultFieldBox(c, s, keyPrefix+"-out", "Output", out)
				}
			})
		}
		if exp != "" {
			ui.Column(c).Grow(1).Width(0).Children(func() {
				if mismatch {
					resultDiffFieldBox(c, s, keyPrefix+"-exp", "Expected", exp, prefix, midExp, suffix, c.Theme().Success)
				} else {
					resultFieldBox(c, s, keyPrefix+"-exp", "Expected", exp)
				}
			})
		}
	})
}

func resultFieldHeader(c *ui.Context, s *judgeState, key, label, raw string, truncated bool) {
	t := c.Theme()
	if s.expanded == nil {
		s.expanded = map[string]bool{}
	}
	expanded := s.expanded[key]
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Grow(1)
		if truncated {
			toggle := fmt.Sprintf("Show all (%d)", len([]rune(raw)))
			if expanded {
				toggle = "Collapse"
			}
			linkButton(c, toggle).Height(22).Padding(0, 6).OnClick(func() {
				s.expanded[key] = !expanded
			})
		}
		iconActionButtonTip(c, "Copy "+label, "Copy", iconDuplicate, t.TextMuted, t.SurfaceHover, tipTop).Size(22, 22).OnClick(func() {
			c.WriteClipboard(raw)
		})
	})
}

func resultFieldBox(c *ui.Context, s *judgeState, key, label, value string) {
	t := c.Theme()
	preview, truncated := truncateResultText(value)
	shown := preview
	if s.expanded != nil && s.expanded[key] {
		shown = value
	}
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		resultFieldHeader(c, s, key, label, value, truncated)
		ui.Column(c).FillWidth().Padding(7, 10).Radius(8).Background(insetBg(c)).Children(func() {
			ui.Text(c, shown).FontSize(12).TextColor(t.Text)
		})
	})
}

func resultDiffFieldBox(c *ui.Context, s *judgeState, key, label, full, prefix, mid, suffix string, accent ui.Color) {
	t := c.Theme()
	preview, truncated := truncateResultText(full)
	expanded := s.expanded != nil && s.expanded[key]
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		resultFieldHeader(c, s, key, label, full, truncated)
		ui.Column(c).FillWidth().Padding(7, 10).Radius(8).Background(insetBg(c)).Children(func() {
			if truncated && !expanded {
				ui.Text(c, preview).FontSize(12).TextColor(t.Text)
				return
			}
			var spans []ui.Span
			if prefix != "" {
				spans = append(spans, ui.Span{Text: prefix, Size: 12, Color: t.Text})
			}
			if mid != "" {
				spans = append(spans, ui.Span{Text: mid, Size: 12, Weight: 600, Color: accent, Background: accent.Alpha(.16), Radius: 3})
			}
			if suffix != "" {
				spans = append(spans, ui.Span{Text: suffix, Size: 12, Color: t.Text})
			}
			if len(spans) == 0 {
				ui.Text(c, full).FontSize(12).TextColor(t.Text)
			} else {
				ui.RichText(c, spans...).FillWidth()
			}
		})
	})
}

func (a *app) resetJudge() {
	if a.judge != nil && a.judge.pending {
		a.judge.cancel()
	}
	a.judge = nil
}

var judgeErrorLine = regexp.MustCompile(`(?i)\bline\s+(\d+)`)
