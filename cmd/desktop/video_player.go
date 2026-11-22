package main

import (
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/render"
)

type linkedImageState struct {
	imgURL  string
	loading bool
	loaded  bool
	bitmap  *ui.Bitmap
}

var (
	reVimeoID   = regexp.MustCompile(`vimeo\.com/(?:video/)?(\d+)`)
	reYouTubeID = regexp.MustCompile(`(?:youtube\.com/(?:embed/|watch\?v=|shorts/)|youtu\.be/)([A-Za-z0-9_-]{6,20})`)
)

func videoProviderName(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "Video"
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case strings.Contains(host, "vimeo.com"):
		return "Vimeo"
	case strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be"):
		return "YouTube"
	case strings.Contains(host, "bilibili.com"):
		return "Bilibili"
	case strings.Contains(host, "wistia."):
		return "Wistia"
	case strings.Contains(host, "loom.com"):
		return "Loom"
	case strings.Contains(host, "leetcode.") || strings.Contains(host, "alicdn.com") || strings.Contains(host, "aliyuncs.com"):
		return "LeetCode Video"
	default:
		return "Video"
	}
}

func (a *app) stopInlineVideo() {
	for k, v := range a.inlineVideos {
		if v != nil {
			v.Destroy()
		}
		delete(a.inlineVideos, k)
	}
}

// ensureInlineVideo eagerly initializes an isolated, ephemeral-session WebView
// directly inside the main window so videos render ready-to-play without an extra click.
// Reusing a single ephemeral session keeps player JS/CSS assets warm in memory across
// videos while remaining strictly isolated from login cookies.
func (a *app) ensureInlineVideo(videoURL string) *mygo.WebView {
	if (!render.IsTrustedVideoHost(videoURL) && !render.IsDirectVideoURL(videoURL)) || a.win == nil || a.closed {
		return nil
	}
	if a.inlineVideos == nil {
		a.inlineVideos = map[string]*mygo.WebView{}
	}
	if v := a.inlineVideos[videoURL]; v != nil && !v.IsDestroyed() {
		return v
	}
	if a.videoSession == nil {
		a.videoSession = mygo.NewEphemeralSession()
	}
	v, err := a.win.NewWebView(mygo.WebViewOptions{
		URL:  "about:blank",
		Page: mygo.PageOptions{Session: a.videoSession, Untrusted: true},
	})
	if err != nil {
		return nil
	}
	_ = v.SetBackgroundColor("#0c0e12")
	v.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
		if e.URL != "" && e.URL != "about:blank" && !strings.HasPrefix(e.URL, "https://") {
			e.PreventDefault()
		}
	})
	v.Page().LoadHTML(buildVideoPlayerHTML(videoURL), a.site.URL()+"/")
	a.inlineVideos[videoURL] = v
	return v
}

func (a *app) renderVideoSolutionCard(c *ui.Context, videoURL, posterURL string) {
	t := c.Theme()
	externalURL := videoURL
	if strings.Contains(strings.ToLower(videoURL), "vimeo.com") && a.problem.Slug != "" {
		externalURL = a.problemURL() + "editorial/"
	}
	title := "Video Solution"
	if a.problem.Title != "" {
		title = a.problem.Title + " — Video Solution"
	}

	view := a.ensureInlineVideo(videoURL)

	ui.Column(c.Key("video-card-"+videoURL)).FillWidth().Radius(12).Border(1, hairlineColor(c)).Background(insetBg(c)).Clip().Children(func() {
		if view != nil {
			ui.WebView(c, view).FillWidth().Height(310)
		} else {
			// Offline/headless preview fallback when running without a native window.
			ui.Box(c).FillWidth().Height(240).Background(ui.Hex("#0c0e12")).Center().Children(func() {
				ui.Column(c).Center().Gap(8).Children(func() {
					ui.Box(c).Size(48, 48).Radius(24).Background(t.Accent).Center().Children(func() {
						ui.Icon(c, iconPlay).FontSize(20).TextColor(t.AccentText).Margin(0, 0, 0, 2)
					})
					ui.Text(c, videoProviderName(videoURL)).FontSize(12).TextColor(ui.Hex("#b8bcc8"))
				})
			})
		}
		ui.Row(c).FillWidth().Padding(9, 14).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(hairlineColor(c)).Children(func() {
			ui.Text(c, title).FontSize(12.5).FontWeight(600).TextColor(t.Text).Grow(1).MinWidth(0).SingleLine()
			iconActionButtonTip(c, "Open in browser", "Open in browser", iconExternal, t.TextMuted, t.SurfaceHover, tipTop).OnClick(func() {
				mygo.Shell.OpenExternal(externalURL)
			})
		})
	})
	_ = posterURL
}

func (a *app) ensureLinkedImage(imgURL string) *linkedImageState {
	if a.linkedImages == nil {
		a.linkedImages = map[string]*linkedImageState{}
	}
	st := a.linkedImages[imgURL]
	if st == nil {
		st = &linkedImageState{imgURL: imgURL}
		a.linkedImages[imgURL] = st
	}
	if !st.loading && !st.loaded && a.post != nil {
		st.loading = true
		go func() {
			bmp, _ := fetchSafeBitmap(imgURL)
			a.post(func() {
				if a.closed {
					return
				}
				st.loading = false
				st.loaded = true
				st.bitmap = bmp
			})
		}()
	}
	return st
}

func (a *app) renderLinkedImageCard(c *ui.Context, linkURL, imgURL, alt string) {
	t := c.Theme()
	st := a.ensureLinkedImage(imgURL)
	label := alt
	if label == "" {
		label = "Open linked image"
	}
	if st.bitmap == nil {
		b := ui.ButtonBase(c).Label(label).Padding(8, 12).Radius(8).Background(insetBg(c)).Border(1, hairlineColor(c))
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconExternal).FontSize(13).TextColor(t.TextMuted)
				ui.Text(c, label).FontSize(12.5).TextColor(accentLinkColor(c)).SingleLine()
			})
		}).OnClick(func() {
			if strings.HasPrefix(linkURL, "https://") || strings.HasPrefix(linkURL, "http://") {
				mygo.Shell.OpenExternal(linkURL)
			}
		})
		return
	}
	iw, ih := st.bitmap.Size()
	btn := ui.ButtonBase(c).Label(label).FillWidth().Radius(10).Clip()
	btn.Children(func() {
		img := ui.Image(c, st.bitmap).FillWidth().Fit(ui.Contain).Radius(10)
		if iw > 0 && ih > 0 {
			img.AspectRatio(float32(iw) / float32(ih))
		}
	}).OnClick(func() {
		if strings.HasPrefix(linkURL, "https://") || strings.HasPrefix(linkURL, "http://") {
			mygo.Shell.OpenExternal(linkURL)
		}
	})
}

func buildVideoPlayerHTML(videoURL string) string {
	safeURL := html.EscapeString(videoURL)
	if render.IsDirectVideoURL(videoURL) {
		return `<!doctype html><html><head><meta charset="utf-8"><style>html,body{margin:0;width:100%;height:100%;background:#0c0e12;display:flex;align-items:center;justify-content:center;overflow:hidden}video{width:100%;height:100%;max-width:100%;max-height:100%;background:#000;outline:none}</style></head><body><video controls preload="metadata" playsinline src="` + safeURL + `"></video></body></html>`
	}
	embedURL := videoURL
	preconnect := ""
	if m := reYouTubeID.FindStringSubmatch(videoURL); len(m) == 2 {
		embedURL = "https://www.youtube-nocookie.com/embed/" + m[1] + "?rel=0&modestbranding=1"
		preconnect = `<link rel="preconnect" href="https://www.youtube-nocookie.com"><link rel="preconnect" href="https://i.ytimg.com">`
	} else if m := reVimeoID.FindStringSubmatch(videoURL); len(m) == 2 {
		embedURL = "https://player.vimeo.com/video/" + m[1] + "?dnt=1&title=0&byline=0&portrait=0"
		preconnect = `<link rel="preconnect" href="https://player.vimeo.com"><link rel="preconnect" href="https://f.vimeocdn.com">`
	} else if u, err := url.Parse(videoURL); err == nil && u.Host != "" {
		preconnect = `<link rel="preconnect" href="https://` + html.EscapeString(u.Host) + `">`
	}
	safeEmbed := html.EscapeString(embedURL)
	return `<!doctype html><html><head><meta charset="utf-8">` + preconnect + `<style>html,body{margin:0;width:100%;height:100%;background:#0c0e12;overflow:hidden}iframe{border:0;width:100%;height:100%;display:block;background:#0c0e12}</style></head><body><iframe src="` + safeEmbed + `" allow="autoplay; fullscreen; picture-in-picture" allowfullscreen referrerpolicy="origin"></iframe></body></html>`
}
