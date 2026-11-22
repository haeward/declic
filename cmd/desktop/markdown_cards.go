package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/render"
)

type playgroundState struct {
	loading  bool
	loaded   bool
	snippets []leetcode.Snippet
}

func (a *app) markdownCodeOptions() *markdown.Options {
	if a.mdCodeOpts == nil {
		a.mdCodeOpts = &markdown.Options{
			WrapCode:       true,
			ShareSelection: true,
		}
	}
	return a.mdCodeOpts
}

func (a *app) ensurePlayground(site leetcode.Site, uuid string) *playgroundState {
	if a.playgrounds == nil {
		a.playgrounds = map[string]*playgroundState{}
	}
	key := string(site) + ":" + uuid
	pg := a.playgrounds[key]
	if pg == nil {
		pg = &playgroundState{}
		a.playgrounds[key] = pg
	}
	if !pg.loading && !pg.loaded && a.post != nil {
		pg.loading = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			snippets, err := leetcode.New(site).PlaygroundCodes(ctx, uuid)
			a.post(func() {
				if a.closed {
					return
				}
				pg.loading = false
				pg.loaded = true
				if err != nil || len(snippets) == 0 {
					return
				}
				pg.snippets = snippets
			})
		}()
	}
	return pg
}

func (a *app) renderMarkdownHTMLBlock(c *ui.Context, rawHTML string) {
	plain := render.ExtractHTMLText(rawHTML)
	if plain == "" {
		return
	}
	ui.Text(c, plain).FontSize(13.5).TextColor(c.Theme().TextMuted)
}

func (a *app) markdownCustomCodeRenderers() map[string]func(c *ui.Context, b *markdown.CodeBlockView) {
	return map[string]func(c *ui.Context, b *markdown.CodeBlockView){
		"declic-video": func(c *ui.Context, b *markdown.CodeBlockView) {
			lines := strings.Split(strings.TrimSpace(b.Code()), "\n")
			if videoURL := strings.TrimSpace(lines[0]); videoURL != "" {
				a.renderVideoSolutionCard(c, videoURL)
			}
		},
		"declic-playground": func(c *ui.Context, b *markdown.CodeBlockView) {
			fields := strings.Fields(b.Code())
			if len(fields) == 2 {
				site := leetcode.Site(fields[0])
				uuid := fields[1]
				if (site == leetcode.COM || site == leetcode.CN) && render.IsSafeSlug(uuid) {
					a.renderPlaygroundCard(c, site, uuid)
				}
			}
		},
		"declic-slides": func(c *ui.Context, b *markdown.CodeBlockView) {
			a.renderSlidesDeckCard(c, strings.TrimSpace(b.Code()))
		},
		"declic-codetabs": func(c *ui.Context, b *markdown.CodeBlockView) {
			var snippets []leetcode.Snippet
			if err := json.Unmarshal([]byte(strings.TrimSpace(b.Code())), &snippets); err == nil && len(snippets) > 0 {
				a.renderCodeTabsCard(c, fmt.Sprintf("tabs-%d-%s", b.Index, snippets[0].Language), snippets, "")
			}
		},
		"declic-linked-image": func(c *ui.Context, b *markdown.CodeBlockView) {
			lines := strings.Split(strings.TrimSpace(b.Code()), "\n")
			if len(lines) < 2 {
				return
			}
			linkURL := strings.TrimSpace(lines[0])
			imgURL := strings.TrimSpace(lines[1])
			alt := ""
			if len(lines) >= 3 {
				alt = strings.TrimSpace(lines[2])
			}
			a.renderLinkedImageCard(c, linkURL, imgURL, alt)
		},
		"mermaid": func(c *ui.Context, b *markdown.CodeBlockView) {
			a.renderMermaidBlock(c, b)
		},
	}
}

func (a *app) renderMermaidBlock(c *ui.Context, b *markdown.CodeBlockView) {
	t := c.Theme()
	src := strings.TrimSpace(b.Code())
	d := render.ParseMermaid(src)

	if a.mermaidShowSrc == nil {
		a.mermaidShowSrc = map[string]bool{}
	}
	showSrc := a.mermaidShowSrc[src]

	ui.Column(c).FillWidth().Radius(12).Border(1, hairlineColor(c)).Background(insetBg(c)).Clip().Children(func() {
		ui.Row(c).FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(hairlineColor(c)).Children(func() {
			ui.Text(c, "Mermaid · "+d.KindLabel).FontSize(12).FontWeight(600).TextColor(t.TextMuted).Grow(1).SingleLine()
			if d.Kind != render.MermaidUnsupported {
				toggleLabel := "Source"
				if showSrc {
					toggleLabel = "Diagram"
				}
				quietButton(c, toggleLabel).Height(24).Padding(0, 8).OnClick(func() {
					a.mermaidShowSrc[src] = !showSrc
				})
			}
			quietButton(c, "Copy").Height(24).Padding(0, 8).OnClick(func() {
				c.WriteClipboard(src)
			})
		})
		if d.Kind == render.MermaidUnsupported || showSrc {
			ui.Column(c).FillWidth().Padding(10, 12).Children(func() {
				markdown.View(c, render.FencedCodeMarkdown("text", src), a.markdownCodeOptions()).FillWidth()
			})
			return
		}
		ui.ScrollHorizontal(c).Scrollbars(false).FillWidth().Children(func() {
			ui.Box(c).FillWidth().Center().Padding(8).Children(func() {
				render.PaintMermaidCanvas(c, d)
			})
		})
	})
}

func (a *app) renderCodeTabsCard(c *ui.Context, key string, snippets []leetcode.Snippet, externalURL string) {
	t := c.Theme()
	if len(snippets) == 0 {
		return
	}
	if a.codeTabSelected == nil {
		a.codeTabSelected = map[string]string{}
	}
	selected := a.codeTabSelected[key]
	active := snippets[0]
	matched := false
	if selected != "" {
		for _, s := range snippets {
			if s.Language == selected {
				active = s
				matched = true
				break
			}
		}
	}
	if !matched {
		for _, s := range snippets {
			if s.Language == a.language {
				active = s
				break
			}
		}
	}

	ui.Column(c.Key("code-tabs-"+key)).FillWidth().Radius(12).Border(1, hairlineColor(c)).Background(insetBg(c)).Clip().Children(func() {
		ui.Row(c).FillWidth().Padding(8, 12).Gap(6).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(hairlineColor(c)).Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).Wrap().Gap(5).AlignItems(ui.Center).Children(func() {
				for _, snip := range snippets {
					lang := snip.Language
					chipButton(c, languageDisplayName(lang), active.Language == lang).OnClick(func() {
						a.codeTabSelected[key] = lang
					})
				}
			})
			if externalURL != "" {
				iconActionButtonTip(c, "Open in Playground", "Open in Playground", iconExternal, t.TextMuted, t.SurfaceHover, tipBottom).OnClick(func() {
					mygo.Shell.OpenExternal(externalURL)
				})
			}
		})
		ui.Column(c).FillWidth().Padding(10, 12).Children(func() {
			markdown.View(c.Key("code-tab-body-"+key+"-"+active.Language), render.FencedCodeMarkdown(active.Language, active.Code), a.markdownCodeOptions()).FillWidth()
		})
	})
}

func (a *app) renderPlaygroundCard(c *ui.Context, site leetcode.Site, uuid string) {
	t := c.Theme()
	pg := a.ensurePlayground(site, uuid)
	playgroundURL := site.URL() + "/playground/" + uuid + "/shared"

	if pg.loading && !pg.loaded {
		ui.Row(c).FillWidth().Padding(12, 16).Gap(12).Radius(12).Background(insetBg(c)).Border(1, hairlineColor(c)).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconCode).FontSize(15).TextColor(t.TextMuted)
			ui.Text(c, "Loading implementation…").FontSize(13).TextColor(t.TextMuted).Grow(1).SingleLine()
			linkButton(c, "Playground ↗").OnClick(func() { mygo.Shell.OpenExternal(playgroundURL) })
		})
		return
	}

	if len(pg.snippets) == 0 {
		ui.Row(c).FillWidth().Padding(12, 16).Gap(12).Radius(12).Background(insetBg(c)).Border(1, hairlineColor(c)).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(34, 34).Radius(17).Background(chipBg(c)).Center().Shrink(0).Children(func() {
				ui.Icon(c, iconCode).FontSize(15).TextColor(t.TextMuted)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, "Implementation Code").FontSize(13).FontWeight(600).TextColor(t.Text).SingleLine()
				ui.Text(c, "Open the interactive code playground on "+string(site)).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
			outlineButton(c, "Open Playground", iconExternal, 30).OnClick(func() {
				mygo.Shell.OpenExternal(playgroundURL)
			})
		})
		return
	}

	a.renderCodeTabsCard(c, "pg-"+uuid, pg.snippets, playgroundURL)
}
