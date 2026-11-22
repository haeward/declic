package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/render"
)

type slideDeckState struct {
	key          string
	loading      bool
	loaded       bool
	failed       bool
	frameURLs    []string
	frames       map[int]*ui.Bitmap
	frameLoading map[int]bool
	index        int
}

func resolveSlideManifestURLs(site leetcode.Site, slug, rawSpec string) []string {
	spec := strings.TrimSpace(rawSpec)
	if colon := strings.LastIndex(spec, ":"); colon > 0 && !strings.HasPrefix(spec[colon:], "://") {
		if strings.Contains(spec[colon+1:], ",") {
			spec = strings.TrimSpace(spec[:colon])
		}
	}
	if spec == "" {
		return nil
	}
	if strings.HasPrefix(spec, "https://") {
		if u, err := url.Parse(spec); err == nil && render.IsTrustedVideoHost(spec) && u.User == nil {
			return []string{spec}
		}
		return nil
	}
	baseEditorial, err := url.Parse(site.URL() + "/problems/" + url.PathEscape(slug) + "/editorial/")
	if err != nil {
		return nil
	}
	var out []string
	add := func(raw string) {
		if raw == "" {
			return
		}
		for _, existing := range out {
			if existing == raw {
				return
			}
		}
		out = append(out, raw)
	}
	if rel, err := url.Parse(spec); err == nil {
		resolved := baseEditorial.ResolveReference(rel)
		if resolved.Scheme == "https" && resolved.Host != "" {
			add(resolved.String())
		}
	}
	trimmed := strings.TrimLeft(spec, "./")
	if trimmed != "" {
		add(site.URL() + "/problems/" + url.PathEscape(slug) + "/" + trimmed)
		add(site.URL() + "/" + trimmed)
		add("https://assets.leetcode.com/static_assets/media/" + trimmed)
	}
	return out
}

func parseSlideManifestFrames(manifestURL string, body []byte) []string {
	var payload struct {
		Timeline []struct {
			Image string `json:"image"`
		} `json:"timeline"`
		Slides []struct {
			Image string `json:"image"`
		} `json:"slides"`
		Frames []string `json:"frames"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	base, err := url.Parse(manifestURL)
	if err != nil {
		return nil
	}
	var rawImages []string
	for _, t := range payload.Timeline {
		if t.Image != "" {
			rawImages = append(rawImages, t.Image)
		}
	}
	if len(rawImages) == 0 {
		for _, s := range payload.Slides {
			if s.Image != "" {
				rawImages = append(rawImages, s.Image)
			}
		}
	}
	if len(rawImages) == 0 {
		rawImages = append(rawImages, payload.Frames...)
	}
	var resolved []string
	for _, img := range rawImages {
		u := strings.TrimSpace(img)
		if u == "" {
			continue
		}
		if strings.HasPrefix(u, "//") {
			u = "https:" + u
		}
		ref, err := url.Parse(u)
		if err != nil {
			continue
		}
		full := base.ResolveReference(ref)
		if full.Scheme != "https" || full.Host == "" || full.User != nil {
			continue
		}
		full.Path = path.Clean(full.Path)
		resolved = append(resolved, full.String())
	}
	return resolved
}

func (a *app) ensureSlideDeck(rawSpec string) *slideDeckState {
	if a.slideDecks == nil {
		a.slideDecks = map[string]*slideDeckState{}
	}
	key := string(a.site) + ":" + a.problem.Slug + ":" + rawSpec
	deck := a.slideDecks[key]
	if deck == nil {
		deck = &slideDeckState{
			key:          key,
			frames:       map[int]*ui.Bitmap{},
			frameLoading: map[int]bool{},
		}
		a.slideDecks[key] = deck
	}
	if !deck.loading && !deck.loaded && a.post != nil {
		deck.loading = true
		candidates := resolveSlideManifestURLs(a.site, a.problem.Slug, rawSpec)
		go func() {
			var frames []string
			for _, u := range candidates {
				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
				if err != nil {
					cancel()
					continue
				}
				req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
				req.Header.Set("Accept", "application/json,*/*;q=0.8")
				resp, err := imageHTTP.Do(req)
				if err != nil {
					cancel()
					continue
				}
				data, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
				status := resp.StatusCode
				resp.Body.Close()
				cancel()
				if readErr == nil && status == http.StatusOK {
					if parsed := parseSlideManifestFrames(u, data); len(parsed) > 0 {
						frames = parsed
						break
					}
				}
			}
			a.post(func() {
				if a.closed {
					return
				}
				deck.loading = false
				deck.loaded = true
				if len(frames) == 0 {
					deck.failed = true
					return
				}
				deck.frameURLs = frames
				a.prefetchSlideFrames(deck, 0)
			})
		}()
	}
	return deck
}

func (a *app) prefetchSlideFrames(deck *slideDeckState, center int) {
	if a.post == nil || len(deck.frameURLs) == 0 {
		return
	}
	for _, idx := range []int{center, center + 1, center - 1} {
		if idx < 0 || idx >= len(deck.frameURLs) {
			continue
		}
		if deck.frames[idx] != nil || deck.frameLoading[idx] {
			continue
		}
		deck.frameLoading[idx] = true
		imgURL := deck.frameURLs[idx]
		frameIdx := idx
		go func() {
			bmp, err := fetchSafeBitmap(imgURL)
			a.post(func() {
				if a.closed {
					return
				}
				deck.frameLoading[frameIdx] = false
				if err == nil && bmp != nil {
					deck.frames[frameIdx] = bmp
				}
			})
		}()
	}
}

func (a *app) renderSlidesDeckCard(c *ui.Context, rawSpec string) {
	t := c.Theme()
	deck := a.ensureSlideDeck(rawSpec)
	editorialURL := a.problemURL() + "editorial/"

	if deck.loading && !deck.loaded {
		ui.Row(c).FillWidth().Padding(12, 16).Gap(12).Radius(12).Background(insetBg(c)).Border(1, hairlineColor(c)).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconEditorial).FontSize(15).TextColor(t.TextMuted)
			ui.Text(c, "Loading interactive walkthrough…").FontSize(13).TextColor(t.TextMuted).Grow(1).SingleLine()
			linkButton(c, "Open ↗").OnClick(func() { mygo.Shell.OpenExternal(editorialURL) })
		})
		return
	}

	if deck.failed || len(deck.frameURLs) == 0 {
		ui.Row(c).FillWidth().Padding(12, 16).Gap(12).Radius(12).Background(insetBg(c)).Border(1, hairlineColor(c)).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(34, 34).Radius(17).Background(chipBg(c)).Center().Shrink(0).Children(func() {
				ui.Icon(c, iconEditorial).FontSize(15).TextColor(t.TextMuted)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, "Interactive Walkthrough").FontSize(13).FontWeight(600).TextColor(t.Text).SingleLine()
				ui.Text(c, "Step-by-step slides are hosted on "+string(a.site)).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
			outlineButton(c, "Open", iconExternal, 30).OnClick(func() {
				mygo.Shell.OpenExternal(editorialURL)
			})
		})
		return
	}

	total := len(deck.frameURLs)
	if deck.index < 0 {
		deck.index = 0
	}
	if deck.index >= total {
		deck.index = total - 1
	}
	a.prefetchSlideFrames(deck, deck.index)
	bmp := deck.frames[deck.index]

	ui.Column(c.Key("slide-deck-"+deck.key)).FillWidth().Radius(12).Border(1, hairlineColor(c)).Background(insetBg(c)).Clip().Children(func() {
		ui.Row(c).FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(hairlineColor(c)).Children(func() {
			ui.Text(c, fmt.Sprintf("Step %d / %d", deck.index+1, total)).FontSize(12).FontWeight(600).TextColor(t.Text).Grow(1).SingleLine()
			outlineButton(c, "Prev", iconChevronLeft, 28).Disabled(deck.index <= 0).OnClick(func() {
				if deck.index > 0 {
					deck.index--
					a.prefetchSlideFrames(deck, deck.index)
				}
			})
			outlineButton(c, "Next", iconChevronRgt, 28).Disabled(deck.index+1 >= total).OnClick(func() {
				if deck.index+1 < total {
					deck.index++
					a.prefetchSlideFrames(deck, deck.index)
				}
			})
			iconActionButtonTip(c, "Open on "+string(a.site), "Open on "+string(a.site), iconExternal, t.TextMuted, t.SurfaceHover, tipBottom).OnClick(func() {
				mygo.Shell.OpenExternal(editorialURL)
			})
		})
		frameBg := t.Surface
		if t.Dark {
			frameBg = ui.Hex("#f4f5f8")
		}
		ui.Box(c).FillWidth().Padding(12).Center().Children(func() {
			if bmp != nil {
				iw, ih := bmp.Size()
				ui.Box(c).FillWidth().Padding(8).Radius(8).Background(frameBg).Center().Children(func() {
					img := ui.Image(c, bmp).FillWidth().Fit(ui.Contain).Radius(6)
					if iw > 0 && ih > 0 {
						img.AspectRatio(float32(iw) / float32(ih))
					}
				})
			} else {
				ui.Box(c).FillWidth().Height(220).Radius(8).Background(t.Surface).Center().Children(func() {
					ui.Text(c, fmt.Sprintf("Loading frame %d…", deck.index+1)).FontSize(12.5).TextColor(t.TextMuted)
				})
			}
		})
	})
}
