package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/render"
	"github.com/haeward/declic/internal/workspace"
)

type resourcePage struct {
	retryArticle                   *leetcode.Article
	retrySubmission                *leetcode.Submission
	loading, loaded, more          bool
	notice                         string
	article                        *leetcode.Article
	rows                           []leetcode.Article
	history                        []leetcode.Submission
	selected                       *leetcode.Submission
	order, language                string
	subStatusFilter, subLangFilter string
	subStatusOpen, subLangOpen     bool
	tocOpen                        bool
	scroll                         ui.ScrollState
	cleanedRaw, cleanedMD          string
	cleanedRefs                    []render.PlaygroundRef
	cleanedVideos                  []string
	cleanedHeadings                []mdHeading
}

var submissionStatuses = []string{
	"Accepted",
	"Wrong Answer",
	"Memory Limit Exceeded",
	"Output Limit Exceeded",
	"Time Limit Exceeded",
	"Runtime Error",
	"Internal Error",
	"Compile Error",
}

func resolveEditorialRelativeURLs(site leetcode.Site, slug, relPath string) []string {
	if site == "" {
		return nil
	}
	base, err := url.Parse(site.URL() + "/problems/" + url.PathEscape(slug) + "/editorial/")
	if err != nil {
		return nil
	}
	var out []string
	add := func(u string) {
		if u == "" || slices.Contains(out, u) {
			return
		}
		out = append(out, u)
	}
	if rel, err := url.Parse(relPath); err == nil && rel.Scheme == "" && rel.Host == "" {
		if resolved := base.ResolveReference(rel); resolved.Scheme == "https" && resolved.Host == base.Host && resolved.User == nil {
			add(resolved.String())
		}
	}
	trimmed := strings.TrimLeft(relPath, "./")
	if trimmed != "" {
		add(site.URL() + "/problems/" + url.PathEscape(slug) + "/" + trimmed)
		add(site.URL() + "/" + trimmed)
		add("https://assets.leetcode.com/static_assets/media/" + trimmed)
	}
	return out
}

func resolveArticleImageURLs(site leetcode.Site, slug, raw string) []string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return nil
	}
	if strings.HasPrefix(cleaned, "//") {
		cleaned = "https:" + cleaned
	}
	u, err := url.Parse(cleaned)
	if err != nil {
		return nil
	}
	if u.Scheme != "" {
		if u.Scheme == "https" && u.Host != "" && u.User == nil {
			return []string{u.String()}
		}
		return nil
	}
	return resolveEditorialRelativeURLs(site, slug, cleaned)
}

type articleContext struct {
	site leetcode.Site
	slug string
}

var activeArticleCtx atomic.Pointer[articleContext]

func (a *app) markdownOptions() *markdown.Options {
	if a.mdOpts == nil {
		a.mdOpts = &markdown.Options{
			Extensions: parser.Default | parser.ExtDollarMath,
			WrapCode:   true,
			LoadImage: func(u string) (ui.ImageSource, error) {
				var site leetcode.Site
				var slug string
				if ctx := activeArticleCtx.Load(); ctx != nil {
					site, slug = ctx.site, ctx.slug
				}
				candidates := resolveArticleImageURLs(site, slug, u)
				if len(candidates) == 0 {
					return nil, fmt.Errorf("unsupported image URL")
				}
				var lastErr error
				for _, cand := range candidates {
					bmp, err := fetchSafeBitmap(cand)
					if err == nil && bmp != nil {
						return bmp, nil
					}
					lastErr = err
				}
				return nil, lastErr
			},
			Math:       render.RenderDisplayMath,
			InlineMath: render.InlineMathObject,
			Code:       a.markdownCustomCodeRenderers(),
			HTML: func(c *ui.Context, rawHTML string) {
				a.renderMarkdownHTMLBlock(c, rawHTML)
			},
		}
	}
	return a.mdOpts
}

func (a *app) resourcePage(tab string) *resourcePage {
	if a.resources == nil {
		a.resources = map[string]*resourcePage{}
	}
	if a.resources[tab] == nil {
		a.resources[tab] = &resourcePage{order: "HOT", language: a.language}
	}
	return a.resources[tab]
}

func (a *app) prepareArticle(tab string, s *resourcePage) {
	if s == nil || s.article == nil || s.article.Content == "" {
		return
	}
	if tab == "Editorial" && !s.article.Visible && s.article.Paid {
		return
	}
	if s.cleanedRaw != s.article.Content {
		s.cleanedRaw = s.article.Content
		s.cleanedMD, s.cleanedRefs, s.cleanedVideos = render.PreprocessArticleMarkdown(s.article.Content)
		s.cleanedHeadings = markdownHeadings(s.cleanedMD)
	}
	for _, ref := range s.cleanedRefs {
		a.ensurePlayground(ref.Site, ref.UUID)
	}
	for _, vid := range s.cleanedVideos {
		a.ensureInlineVideo(vid)
	}
}

func (a *app) prefetchEditorial(site leetcode.Site, slug string) {
	if slug == "" || a.closed {
		return
	}
	s := a.resourcePage("Editorial")
	if s.loaded {
		a.prepareArticle("Editorial", s)
		return
	}
	if s.loading {
		return
	}
	storeRoot := a.store.Root
	user := a.username
	go func() {
		if storeRoot != "" && user == "" {
			if cached, ok := readEditorialCache(storeRoot, string(site), slug); ok {
				a.post(func() {
					if a.closed || a.resources["Editorial"] != s || a.site != site || a.problem.Slug != slug || s.loaded {
						return
					}
					s.article = cached
					s.loaded = true
					a.prepareArticle("Editorial", s)
				})
				return
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		client := leetcode.New(site)
		var cookies []*http.Cookie
		var err error
		if user != "" {
			cookies, _, err = verifiedCookiesCached(ctx, client, user)
		}
		if err != nil {
			return
		}
		art, err := client.EditorialForViewer(ctx, slug, cookies)
		if err != nil {
			return
		}
		if storeRoot != "" && (art == nil || (!art.Paid && art.Visible)) {
			writeEditorialCache(storeRoot, string(site), slug, art)
		}
		a.post(func() {
			if a.closed || a.resources["Editorial"] != s || a.site != site || a.username != user || a.problem.Slug != slug || s.loaded {
				return
			}
			s.article = art
			s.loaded = true
			a.prepareArticle("Editorial", s)
		})
	}()
}

func (a *app) loadResource(tab string, article *leetcode.Article, submission *leetcode.Submission) {
	s := a.resourcePage(tab)
	if s.loading || tab == "Submissions" && a.username == "" {
		return
	}
	// Check public Editorial disk cache before hitting the network.
	if tab == "Editorial" && article == nil && submission == nil && a.store.Root != "" && a.username == "" {
		if cached, ok := readEditorialCache(a.store.Root, string(a.site), a.problem.Slug); ok {
			s.article = cached
			s.loaded = true
			a.prepareArticle("Editorial", s)
			return
		}
	}
	s.retryArticle, s.retrySubmission = article, submission
	s.loading = true
	s.notice = ""
	site, user, slug, order, language := a.site, a.username, a.problem.Slug, s.order, s.language
	storeRoot := a.store.Root
	offset := len(s.rows)
	if tab == "Submissions" {
		offset = len(s.history)
	}
	go func() {
		client := leetcode.New(site)
		ctx := context.Background()
		var err error
		var detail *leetcode.Article
		var history []leetcode.Submission
		var rows []leetcode.Article
		var code string
		var more bool
		switch tab {
		case "Editorial":
			var cookies []*http.Cookie
			if user != "" {
				cookies, _, err = verifiedCookiesCached(ctx, client, user)
			}
			if err == nil {
				detail, err = client.EditorialForViewer(ctx, slug, cookies)
			}
			if err == nil && storeRoot != "" && (detail == nil || (!detail.Paid && detail.Visible)) {
				writeEditorialCache(storeRoot, string(site), slug, detail)
			}
		case "Solutions":
			if article != nil {
				detail, err = client.Solution(ctx, article.Slug)
			} else {
				rows, err = client.Solutions(ctx, slug, order, language, offset)
				more = len(rows) == 20
			}
		case "Submissions":
			var cookies []*http.Cookie
			cookies, _, err = verifiedCookiesCached(ctx, client, user)
			if err == nil {
				if submission == nil {
					history, more, err = client.Submissions(ctx, slug, offset, cookies)
				} else {
					var id int
					id, err = strconv.Atoi(submission.ID)
					if err == nil {
						code, err = client.SubmissionCode(ctx, id, cookies)
					}
				}
			}
		}
		a.post(func() {
			if a.closed || a.resources[tab] != s || a.site != site || a.username != user || a.problem.Slug != slug {
				return
			}
			s.loaded = true
			s.loading = false
			if err != nil {
				s.notice = err.Error()
				if err == errSessionExpired {
					a.expireSession()
				}
				return
			}
			if submission != nil {
				sub := *submission
				sub.Code = code
				s.selected = &sub
				return
			}
			if detail != nil || tab == "Editorial" || article != nil {
				if detail != nil && article != nil {
					if detail.Author.Name == "" {
						detail.Author = article.Author
					}
					if detail.Views == 0 {
						detail.Views = article.Views
					}
					if detail.Votes == nil {
						detail.Votes = article.Votes
					}
				}
				s.article = detail
				a.prepareArticle(tab, s)
				return
			}
			s.more = more
			s.rows = append(s.rows, rows...)
			s.history = append(s.history, history...)
		})
	}()
}

func (a *app) resourceView(c *ui.Context) {
	tab := a.resourceTab
	s := a.resourcePage(tab)
	if a.expired && (tab == "Submissions" || tab == "Editorial") {
		emptyStateAction(c, iconAccount, "Session expired", "Your "+string(a.site)+" session has expired. Sign in again to restore access.", "Sign in again", a.openLogin)
		return
	}
	if tab == "Submissions" && a.username == "" {
		emptyStateAction(c, iconAccount, "Sign in to view submissions", "Connect your "+string(a.site)+" account to view submission history, runtime metrics, and saved code.", "Sign in", a.openLogin)
		return
	}
	if !s.loaded && !s.loading {
		a.loadResource(tab, nil, nil)
	}
	// Seamless lazy / infinite loading: when scrolled near the bottom of Solutions or Submissions,
	// fetch the next page in the background without clearing or jumping the existing list.
	if s.more && !s.loading && s.notice == "" && s.article == nil && s.selected == nil &&
		((tab == "Solutions" && len(s.rows) > 0) || (tab == "Submissions" && len(s.history) > 0)) &&
		s.scroll.MaxY > 0 && s.scroll.Y >= s.scroll.MaxY-160 {
		a.loadResource(tab, nil, nil)
	}
	t := c.Theme()
	hasListItems := (tab == "Solutions" && len(s.rows) > 0) || (tab == "Submissions" && len(s.history) > 0)
	loadingDetail := s.loading && (s.retryArticle != nil || s.retrySubmission != nil)
	ui.Scroll(c.Key(tab)).Scrollbars(false).TrackScroll(&s.scroll).Fill().Children(func() {
		ui.Column(c).FillWidth().Children(func() {
			if (tab == "Editorial" || tab == "Solutions") && a.problem.ID != "" {
				a.problemContextHeader(c, false)
				ui.Box(c).FillWidth().Height(1).Background(hairlineColor(c))
			}
			if s.notice != "" && (!hasListItems || s.retryArticle != nil || s.retrySubmission != nil) {
				ui.Column(c).FillWidth().Padding(18, 22).Gap(10).AlignItems(ui.Start).Children(func() {
					ui.Text(c, "Could not load "+tab).Bold()
					ui.Text(c, s.notice).TextColor(t.TextMuted)
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						outlineButton(c, "Retry", nil, 30).OnClick(func() { a.loadResource(tab, s.retryArticle, s.retrySubmission) })
						ui.Link(c, "Open on LeetCode ↗", a.problemURL()+map[string]string{"Submissions": "submissions/", "Solutions": "solutions/", "Editorial": "editorial/"}[tab])
					})
				})
				return
			}
			if s.loading && (!hasListItems || loadingDetail) && !(tab == "Solutions" && s.article == nil && !loadingDetail) {
				ui.Column(c).FillWidth().Height(240).Center().Children(func() {
					ui.Text(c, "Loading "+strings.ToLower(tab)+"…").FontSize(13).TextColor(t.TextMuted)
				})
				return
			}
			if s.selected != nil {
				sub := s.selected
				statusColor := t.Danger
				if sub.Status == "Accepted" {
					statusColor = t.Success
				}
				stamp, _ := strconv.ParseInt(sub.Timestamp, 10, 64)
				meta := []string{languageDisplayName(sub.Language)}
				if sub.Runtime != "" {
					meta = append(meta, sub.Runtime)
				}
				if sub.Memory != "" {
					meta = append(meta, sub.Memory)
				}
				if stamp > 0 {
					meta = append(meta, time.Unix(stamp, 0).Format("Jan 2"))
				}
				ui.Column(c).FillWidth().Padding(14, 20).Gap(12).AlignItems(ui.Start).Children(func() {
					linkButton(c, "‹ All submissions").OnClick(func() { s.selected = nil })
					ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, sub.Status).FontSize(18).Bold().TextColor(statusColor)
						ui.Text(c, joinDot(meta)).FontSize(12.5).TextColor(t.TextMuted).SingleLine()
					})
					ui.Column(c).FillWidth().Children(func() {
						markdown.View(c.Key("sub-code-"+sub.ID), render.FencedCodeMarkdown(sub.Language, sub.Code), a.markdownCodeOptions()).FillWidth()
					})
					ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.Center).Children(func() {
						btnLabel := "Use this code"
						var btnIcon *ui.SVG
						if time.Now().Before(a.subUsedUntil) {
							btnLabel = "Loaded"
							btnIcon = iconCheck
						}
						useBtn := primaryActionButton(c, btnLabel, btnIcon, 32).Disabled(a.busy || sub.Code == "").OnClick(func() {
							a.useSubmission(*sub)
							a.subUsedUntil = time.Now().Add(1500 * time.Millisecond)
						})
						attachTooltip(c, useBtn, "Loads into the matching language. Your current code is kept as an undo checkpoint.", tipTop)
					})
				})
				return
			}
			if s.article != nil {
				ui.Column(c).FillWidth().Padding(14, 22, 24, 22).Gap(12).AlignItems(ui.Start).Children(func() {
					if tab == "Solutions" {
						linkButton(c, "‹ All solutions").OnClick(func() { a.pauseInlineVideos(); s.article = nil })
					}
					if s.article.Title != "" && (tab != "Editorial" || !strings.EqualFold(strings.TrimSpace(s.article.Title), strings.TrimSpace(a.problem.Title))) {
						ui.Text(c, s.article.Title).FontSize(18).Bold()
					}
					if tab == "Solutions" && (s.article.Author.Name != "" || s.article.Views > 0) {
						parts := []string{s.article.Author.Name}
						if s.article.Votes != nil {
							parts = append(parts, fmt.Sprintf("▲ %s", compactCount(*s.article.Votes)))
						}
						if s.article.Views > 0 {
							parts = append(parts, fmt.Sprintf("%s views", compactCount(s.article.Views)))
						}
						ui.Text(c, joinDot(parts)).FontSize(12).TextColor(t.TextMuted).SingleLine()
					}
					if tab == "Editorial" && !s.article.Visible && (s.article.Paid || s.article.Content == "") {
						emptyStateAction(c, iconLock, "Premium Editorial", "This official editorial requires a LeetCode Premium subscription.", "Open on "+string(a.site), func() {
							mygo.Shell.OpenExternal(a.problemURL() + "editorial/")
						})
						return
					}
					activeArticleCtx.Store(&articleContext{site: a.site, slug: a.problem.Slug})
					opts := a.markdownOptions()
					a.prepareArticle(tab, s)
					if len(s.cleanedHeadings) >= 2 {
						a.markdownTOC(c, s, s.cleanedHeadings, opts)
					}
					markdown.View(c, s.cleanedMD, opts).FillWidth()
				})
				return
			}
			switch tab {
			case "Editorial":
				nullEmptyState(c, "No official editorial yet", "Check the Solutions tab for community approaches and explanations.")
			case "Solutions":
				ui.Row(c).FillWidth().Height(44).Padding(0, 20).Gap(4).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(hairlineColor(c)).Children(func() {
					for _, o := range []struct{ key, label string }{{"HOT", "Hot"}, {"NEWEST", "Newest"}, {"MOST_VOTES", "Most votes"}} {
						opt := o
						active := s.order == opt.key
						btn := ui.ButtonBase(c).Label(opt.label).Height(26).Padding(0, 10).Radius(7).AlignItems(ui.Center)
						fg, weight := t.TextMuted, 500
						if active {
							btn.Background(chipBg(c))
							fg, weight = t.Text, 600
						} else if btn.Hovered() {
							btn.Background(t.SurfaceHover)
							fg = t.Text
						}
						applyPressFeedback(btn, 0.96)
						btn.Children(func() {
							ui.Text(c, opt.label).FontSize(12.5).FontWeight(weight).TextColor(fg).SingleLine()
						}).OnClick(func() {
							if s.order != opt.key {
								s.order = opt.key
								s.rows = nil
								s.more = false
								s.notice = ""
								a.loadResource(tab, nil, nil)
							}
						})
					}
					ui.Box(c).Grow(1)
					langActive := s.language != ""
					langName := languageDisplayName(a.language)
					langBtn := ui.ButtonBase(c).Label(langName).Height(26).Padding(0, 10).Radius(13).AlignItems(ui.Center)
					langFg := t.TextMuted
					if langActive {
						langBtn.Background(activeChipBg(c))
						langFg = accentLinkColor(c)
					} else {
						langBtn.Border(1, t.Border.Alpha(.8))
						if langBtn.Hovered() {
							langBtn.Background(t.SurfaceHover)
							langFg = t.Text
						}
					}
					applyPressFeedback(langBtn, 0.96)
					langBtn.Children(func() {
						ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, iconCode).FontSize(12).TextColor(langFg)
							weight := 500
							if langActive {
								weight = 600
							}
							ui.Text(c, langName).FontSize(12).FontWeight(weight).TextColor(langFg).SingleLine()
							if langActive {
								ui.Icon(c, iconClose).FontSize(10).TextColor(langFg.Alpha(.8))
							}
						})
					}).OnClick(func() {
						if s.language == "" {
							s.language = a.language
						} else {
							s.language = ""
						}
						s.rows = nil
						s.more = false
						s.notice = ""
						a.loadResource(tab, nil, nil)
					})
				})
				if s.loading && len(s.rows) == 0 {
					ui.Column(c).FillWidth().Height(220).Center().Children(func() {
						ui.Text(c, "Loading solutions…").FontSize(13).TextColor(t.TextMuted)
					})
					break
				}
				for i, r := range s.rows {
					row := r
					if i > 0 {
						insetHairline(c, hairlineColor(c), 0, 20, 0)
					}
					b := ui.ButtonBase(c).Label(row.Title).FillWidth().Padding(12, 20)
					hovered := b.Hovered()
					if hovered {
						b.Background(t.SurfaceHover)
					}
					b.Children(func() {
						ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
							authorName := strings.TrimSpace(row.Author.Name)
							ui.Column(c).Grow(1).MinWidth(0).Gap(5).AlignItems(ui.Start).Children(func() {
								titleFg := t.Text
								if hovered {
									titleFg = accentLinkColor(c)
								}
								ui.Text(c, strings.TrimSpace(row.Title)).FontSize(13.5).FontWeight(600).TextColor(titleFg).SingleLine()
								ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
									if authorName != "" {
										ui.Row(c).Gap(5).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
											ui.Icon(c, iconAccount).FontSize(11.5).TextColor(t.TextMuted.Alpha(.8))
											ui.Text(c, authorName).FontSize(12).TextColor(t.TextMuted).Shrink(1).MinWidth(0).SingleLine()
										})
									}
									if row.Votes != nil {
										ui.Row(c).Gap(3).AlignItems(ui.Center).Shrink(0).Children(func() {
											ui.Icon(c, iconUpvote).FontSize(11.5).TextColor(t.TextMuted.Alpha(.8))
											ui.Text(c, compactCount(*row.Votes)).FontSize(12).TextColor(t.TextMuted).SingleLine()
										})
									}
									ui.Row(c).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
										ui.Icon(c, iconEye).FontSize(12).TextColor(t.TextMuted.Alpha(.8))
										ui.Text(c, compactCount(row.Views)).FontSize(12).TextColor(t.TextMuted).SingleLine()
									})
								})
							})
							chevColor := t.TextMuted.Alpha(.42)
							if hovered {
								chevColor = t.TextMuted
							}
							ui.Icon(c, iconChevronRgt).FontSize(13).TextColor(chevColor).Shrink(0)
						})
					})
					b.OnClick(func() { a.loadResource(tab, &row, nil) })
				}
				if len(s.rows) == 0 {
					if s.language != "" {
						nullEmptyState(c, "No matching solutions", "Try clearing the language filter or switching the sort order.")
					} else {
						nullEmptyState(c, "No community solutions yet", "Be the first to share an approach on "+string(a.site)+".")
					}
				}
			case "Submissions":
				a.submissionsHeader(c, s)
				filtered := filterSubmissions(s.history, s.subStatusFilter, s.subLangFilter)
				for _, r := range filtered {
					row := r
					stamp, _ := strconv.ParseInt(row.Timestamp, 10, 64)
					color := t.Danger
					if row.Status == "Accepted" {
						color = t.Success
					}
					b := ui.ButtonBase(c).Label(row.Status+" · "+row.ID).FillWidth().Height(42).Padding(0, 20)
					if b.Hovered() {
						b.Background(t.SurfaceHover)
					}
					b.Children(func() {
						submissionRow(c, []string{row.Status, languageDisplayName(row.Language), row.Runtime, row.Memory, time.Unix(stamp, 0).Format("Jan 2")}, color)
					}).OnClick(func() { a.loadResource(tab, nil, &row) })
				}
				if len(filtered) == 0 {
					if len(s.history) == 0 {
						nullEmptyState(c, "No submissions yet", "Submit your solution to track verdicts, runtime, and memory.")
					} else {
						nullEmptyState(c, "No matching submissions", "Try clearing the status or language filter to see all attempts.")
					}
				}
			}
			if s.notice != "" && hasListItems {
				ui.Row(c).FillWidth().Padding(12, 20).Gap(10).Center().Children(func() {
					ui.Text(c, s.notice).FontSize(12).TextColor(t.TextMuted).SingleLine()
					linkButton(c, "Retry").OnClick(func() { a.loadResource(tab, nil, nil) })
				})
			} else if s.loading && hasListItems {
				ui.Row(c).FillWidth().Padding(14, 18).Center().Children(func() {
					ui.Text(c, "Loading more…").FontSize(12).TextColor(t.TextMuted)
				})
			} else if s.more {
				ui.Row(c).FillWidth().Padding(14, 18).Center().Children(func() {
					linkButton(c, "Load more").OnClick(func() { a.loadResource(tab, nil, nil) })
				})
			}
		})
	})
}

func (a *app) submissionsHeader(c *ui.Context, s *resourcePage) {
	t := c.Theme()
	weights := []float32{1.5, 0.9, 0.65, 0.8, 0.85}
	ui.Row(c).FillWidth().Height(38).Padding(0, 20).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(hairlineColor(c)).Children(func() {
		// Status filter dropdown column
		statusLabel := "Status"
		if s.subStatusFilter != "" {
			statusLabel = s.subStatusFilter
		}
		statusAnchor := ui.ButtonBase(c).Label("Filter status").Width(0).Grow(weights[0]).Height(28).Radius(6)
		if statusAnchor.Hovered() {
			statusAnchor.Background(t.SurfaceHover)
		}
		statusAnchor.Children(func() {
			ui.Row(c).Fill().Gap(4).AlignItems(ui.Center).Children(func() {
				fg := t.TextMuted
				if s.subStatusFilter != "" {
					fg = t.Text
				}
				ui.Text(c, statusLabel).FontSize(12).FontWeight(500).TextColor(fg).Shrink(1).MinWidth(0).SingleLine()
				chev := iconChevronDown
				if s.subStatusOpen {
					chev = iconChevronUp
				}
				ui.Icon(c, chev).FontSize(12).TextColor(t.TextMuted)
			})
		}).OnClick(func() { s.subStatusOpen = !s.subStatusOpen; s.subLangOpen = false })
		if s.subStatusOpen {
			ui.PopoverBase(c, statusAnchor, &s.subStatusOpen, func(panel ui.Element) {
				panel.Width(216).Padding(6).Radius(10).Background(t.Surface).Border(1, t.Border.Alpha(.55)).Shadow(0, 8, 26, 0, ui.RGBA(0, 0, 0, .20))
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					items := append([]string{"All statuses"}, submissionStatuses...)
					for _, label := range items {
						val := label
						if label == "All statuses" {
							val = ""
						}
						selected := s.subStatusFilter == val
						b := ui.ButtonBase(c).Label(label).FillWidth().Height(30).Padding(0, 10).Radius(7)
						if b.Hovered() {
							b.Background(t.SurfaceHover)
						}
						b.Children(func() {
							ui.Row(c).Fill().Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Text(c, label).FontSize(12.5).TextColor(t.Text).Grow(1).MinWidth(0).SingleLine()
								if selected {
									ui.Icon(c, iconCheck).FontSize(12).TextColor(accentLinkColor(c))
								}
							})
						}).OnClick(func() { s.subStatusFilter = val; s.subStatusOpen = false })
					}
				})
			})
		}

		// Language filter dropdown column
		langLabel := "Language"
		if s.subLangFilter != "" {
			langLabel = languageDisplayName(s.subLangFilter)
		}
		langAnchor := ui.ButtonBase(c).Label("Filter language").Width(0).Grow(weights[1]).Height(28).Radius(6)
		if langAnchor.Hovered() {
			langAnchor.Background(t.SurfaceHover)
		}
		langAnchor.Children(func() {
			ui.Row(c).Fill().Gap(4).AlignItems(ui.Center).Children(func() {
				fg := t.TextMuted
				if s.subLangFilter != "" {
					fg = t.Text
				}
				ui.Text(c, langLabel).FontSize(12).FontWeight(500).TextColor(fg).Shrink(1).MinWidth(0).SingleLine()
				chev := iconChevronDown
				if s.subLangOpen {
					chev = iconChevronUp
				}
				ui.Icon(c, chev).FontSize(12).TextColor(t.TextMuted)
			})
		}).OnClick(func() { s.subLangOpen = !s.subLangOpen; s.subStatusOpen = false })
		if s.subLangOpen {
			var langs []string
			for _, sub := range s.history {
				if sub.Language != "" && !slices.Contains(langs, sub.Language) {
					langs = append(langs, sub.Language)
				}
			}
			slices.Sort(langs)
			ui.PopoverBase(c, langAnchor, &s.subLangOpen, func(panel ui.Element) {
				panel.Width(176).Padding(6).Radius(10).Background(t.Surface).Border(1, t.Border.Alpha(.55)).Shadow(0, 8, 26, 0, ui.RGBA(0, 0, 0, .20))
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					allBtn := ui.ButtonBase(c).Label("All languages").FillWidth().Height(30).Padding(0, 10).Radius(7)
					if allBtn.Hovered() {
						allBtn.Background(t.SurfaceHover)
					}
					allBtn.Children(func() {
						ui.Row(c).Fill().Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "All languages").FontSize(12.5).TextColor(t.Text).Grow(1)
							if s.subLangFilter == "" {
								ui.Icon(c, iconCheck).FontSize(12).TextColor(accentLinkColor(c))
							}
						})
					}).OnClick(func() { s.subLangFilter = ""; s.subLangOpen = false })
					for _, slug := range langs {
						lang := slug
						display := languageDisplayName(lang)
						b := ui.ButtonBase(c).Label(display).FillWidth().Height(30).Padding(0, 10).Radius(7)
						if b.Hovered() {
							b.Background(t.SurfaceHover)
						}
						b.Children(func() {
							ui.Row(c).Fill().Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Text(c, display).FontSize(12.5).TextColor(t.Text).Grow(1)
								if s.subLangFilter == lang {
									ui.Icon(c, iconCheck).FontSize(12).TextColor(accentLinkColor(c))
								}
							})
						}).OnClick(func() { s.subLangFilter = lang; s.subLangOpen = false })
					}
				})
			})
		}

		for i, col := range []string{"Runtime", "Memory", "Submitted"} {
			ui.Text(c, col).Width(0).Grow(weights[i+2]).TextAlign(ui.End).FontSize(12).TextColor(t.TextMuted).SingleLine()
		}
	})
}

func filterSubmissions(rows []leetcode.Submission, status, lang string) []leetcode.Submission {
	if status == "" && lang == "" {
		return rows
	}
	var out []leetcode.Submission
	for _, r := range rows {
		if status != "" && r.Status != status {
			continue
		}
		if lang != "" && r.Language != lang {
			continue
		}
		out = append(out, r)
	}
	return out
}

type mdHeading struct {
	Level int
	Text  string
	Slug  string
}

func markdownHeadings(src string) []mdHeading {
	var out []mdHeading
	inFence := false
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		level := 0
		if strings.HasPrefix(trim, "## ") {
			level = 2
			trim = strings.TrimSpace(trim[3:])
		} else if strings.HasPrefix(trim, "### ") {
			level = 3
			trim = strings.TrimSpace(trim[4:])
		}
		if level > 0 && trim != "" {
			slug := headingSlug(trim)
			if slug != "" {
				out = append(out, mdHeading{Level: level, Text: trim, Slug: slug})
			}
		}
	}
	return out
}

func headingSlug(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if (r == ' ' || r == '-' || r == '_') && !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func (a *app) markdownTOC(c *ui.Context, s *resourcePage, headings []mdHeading, opts *markdown.Options) {
	t := c.Theme()
	ui.Column(c).FillWidth().Padding(8, 12).Gap(6).Radius(10).Background(insetBg(c)).Border(1, hairlineColor(c)).Children(func() {
		chev := iconChevronDown
		if s.tocOpen {
			chev = iconChevronUp
		}
		ui.ButtonBase(c).Label("Contents").FillWidth().Height(24).Children(func() {
			ui.Row(c).Fill().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconProblems).FontSize(13).TextColor(t.TextMuted)
					ui.Text(c, fmt.Sprintf("Contents (%d)", len(headings))).FontSize(12).FontWeight(600).TextColor(t.Text)
				})
				ui.Icon(c, chev).FontSize(13).TextColor(t.TextMuted)
			})
		}).OnClick(func() { s.tocOpen = !s.tocOpen })
		if s.tocOpen {
			ui.Column(c).FillWidth().Gap(2).Padding(2, 0, 4, 0).Children(func() {
				for _, h := range headings {
					item := h
					leftPad := float32(6)
					if item.Level == 3 {
						leftPad = 18
					}
					b := ui.ButtonBase(c).Label(item.Text).FillWidth().Height(26).Padding(0, 8, 0, leftPad).Radius(6)
					if b.Hovered() {
						b.Background(t.SurfaceHover)
					}
					b.Children(func() {
						ui.Row(c).Fill().AlignItems(ui.Center).Children(func() {
							ui.Text(c, item.Text).FontSize(12).TextColor(t.TextMuted).SingleLine()
						})
					}).OnClick(func() {
						opts.Anchor = item.Slug
						opts.AnchorVersion++
					})
				}
			})
		}
	})
}

func (a *app) useSubmission(sub leetcode.Submission) bool {
	for _, snippet := range a.problem.Snippets {
		if snippet.Language == sub.Language && workspace.Extension(sub.Language) != "" {
			if a.language == sub.Language {
				a.code.Buffer.Set(sub.Code)
				return true
			}
			a.save(func() {
				a.loadIdentityWithCode(a.site, a.account, a.username, a.problem, sub.Language, "workspace", &sub.Code)
			})
			return true
		}
	}
	a.status = "This submission language is unavailable in the editor."
	a.errorStatus = true
	return false
}

func submissionRow(c *ui.Context, columns []string, status ui.Color) {
	t := c.Theme()
	weights := []float32{1.5, 0.9, 0.65, 0.8, 0.85}
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		for i, value := range columns {
			color := t.Text
			fontWeight := 400
			if i == 0 {
				color = status
				fontWeight = 600
			} else if i == 4 || value == "N/A" {
				color = t.TextMuted
			}
			el := ui.Text(c, value).Width(0).Grow(weights[i]).SingleLine().FontSize(13).FontWeight(fontWeight).TextColor(color)
			attachTooltip(c, el, value, tipTop)
			if i >= 2 {
				el.TextAlign(ui.End)
			}
		}
	})
}

func joinDot(parts []string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += " · "
		}
		out += p
	}
	return out
}

func compactCount(n int) string {
	if n >= 1000 {
		v := float64(n) / 1000
		if v >= 10 {
			return fmt.Sprintf("%.0fK", v)
		}
		return fmt.Sprintf("%.1fK", v)
	}
	return fmt.Sprint(n)
}
