package main

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/render"
)

type linkedImageState struct {
	loading bool
	loaded  bool
	bitmap  *ui.Bitmap
}

var (
	reVimeoPath = regexp.MustCompile(`^/(?:video/)?([0-9]+)/?$`)
	reYouTubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{6,20}$`)
)

func parseVideoURL(raw string) (*url.URL, string) {
	cleaned := strings.TrimSpace(raw)
	if strings.HasPrefix(cleaned, "//") {
		cleaned = "https:" + cleaned
	}
	u, err := url.Parse(cleaned)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, ""
	}
	return u, strings.ToLower(u.Hostname())
}

func hostMatches(host string, domains ...string) bool {
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

func (a *app) stopInlineVideo() {
	for k, v := range a.inlineVideos {
		if v != nil {
			v.Destroy()
		}
		delete(a.inlineVideos, k)
	}
}

// pauseInlineVideos pauses and mutes active playback when switching tabs within
// the same problem, keeping loaded WebViews alive so switching between Editorial
// and Solutions never reloads videos.
func (a *app) pauseInlineVideos() {
	const pauseJS = `(function(){var v=document.querySelector('video');if(v&&!v.paused)v.pause();var f=document.getElementById('p');if(f&&f.contentWindow){f.contentWindow.postMessage('{"method":"pause"}','https://player.vimeo.com');f.contentWindow.postMessage('{"event":"command","func":"pauseVideo","args":""}','https://www.youtube-nocookie.com');}})()`
	for k, v := range a.inlineVideos {
		if v == nil || v.IsDestroyed() {
			delete(a.inlineVideos, k)
			continue
		}
		page := v.Page()
		page.SetAudioMuted(true)
		go func(p *mygo.Page) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = p.EvalContext(ctx, pauseJS)
		}(page)
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
	// Cap resident WebViews per problem by evicting a single entry rather than clearing all.
	if len(a.inlineVideos) >= 6 {
		for k, old := range a.inlineVideos {
			if old != nil {
				old.Destroy()
			}
			delete(a.inlineVideos, k)
			break
		}
	}
	if a.videoSession == nil {
		a.videoSession = mygo.NewEphemeralSession()
	}
	v, err := a.win.NewWebView(mygo.WebViewOptions{
		Page: mygo.PageOptions{
			Session:              a.videoSession,
			Untrusted:            true,
			BackgroundThrottling: mygo.ThrottlingOff,
		},
	})
	if err != nil {
		return nil
	}
	_ = v.SetBackgroundColor("#0c0e12")
	v.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
		if e.URL != "" && e.URL != "about:blank" && !render.IsTrustedVideoHost(e.URL) && !render.IsDirectVideoURL(e.URL) {
			e.PreventDefault()
		}
	})
	v.Page().LoadHTML(buildVideoPlayerHTML(videoURL), a.site.URL()+"/")
	a.inlineVideos[videoURL] = v
	return v
}

// renderPrewarmedVideos keeps inactive inline video WebViews mounted behind the
// opaque workspace surface at a real 16:9 viewport (640x360) inside a 1x1
// zero-opacity clip so macOS WKWebView runs at full priority and Vimeo/YouTube
// finish booting and fetching HD posters while reading Description or browsing Solutions.
func (a *app) renderPrewarmedVideos(c *ui.Context) {
	if len(a.inlineVideos) == 0 {
		return
	}
	active := map[string]bool{}
	if a.resourceTab == "Editorial" || a.resourceTab == "Solutions" {
		if s := a.resources[a.resourceTab]; s != nil && s.article != nil {
			for _, u := range s.cleanedVideos {
				active[u] = true
			}
		}
	}
	keys := make([]string, 0, len(a.inlineVideos))
	for u, v := range a.inlineVideos {
		if v == nil || v.IsDestroyed() || active[u] {
			continue
		}
		keys = append(keys, u)
	}
	if len(keys) == 0 {
		return
	}
	sort.Strings(keys)
	ui.Box(c).Absolute().Top(0).Left(0).Size(1, 1).Clip().Opacity(0).IgnoreInput(true).Children(func() {
		for _, k := range keys {
			ui.WebView(c.Key("video-warm-"+k), a.inlineVideos[k]).Size(800, 450)
		}
	})
}

func (a *app) renderVideoSolutionCard(c *ui.Context, videoURL string) {
	if !render.IsTrustedVideoHost(videoURL) && !render.IsDirectVideoURL(videoURL) {
		return
	}
	if view := a.ensureInlineVideo(videoURL); view != nil {
		if view.Page().IsAudioMuted() {
			view.Page().SetAudioMuted(false)
		}
		ui.WebView(c.Key("video-card-"+videoURL), view).FillWidth().AspectRatio(16.0 / 9.0).Radius(12)
		return
	}
	t := c.Theme()
	// Offline/headless preview fallback when running without a native window.
	ui.Box(c.Key("video-card-" + videoURL)).FillWidth().AspectRatio(16.0 / 9.0).Radius(12).Background(ui.Hex("#0c0e12")).Center().Children(func() {
		ui.Column(c).Center().Gap(8).Children(func() {
			ui.Box(c).Size(48, 48).Radius(24).Background(t.Accent).Center().Children(func() {
				ui.Icon(c, iconPlay).FontSize(20).TextColor(t.AccentText).Margin(0, 0, 0, 2)
			})
			ui.Text(c, "Loading video…").FontSize(12).TextColor(ui.Hex("#b8bcc8"))
		})
	})
}

func (a *app) ensureLinkedImage(imgURL string) *linkedImageState {
	if a.linkedImages == nil {
		a.linkedImages = map[string]*linkedImageState{}
	}
	st := a.linkedImages[imgURL]
	if st == nil {
		st = &linkedImageState{}
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

func openSafeExternalURL(raw string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil {
		mygo.Shell.OpenExternal(u.String())
	}
}

func (a *app) renderLinkedImageCard(c *ui.Context, linkURL, imgURL, alt string) {
	t := c.Theme()
	st := a.ensureLinkedImage(imgURL)
	label := alt
	if label == "" {
		label = "Open linked image"
	}
	openLink := func() { openSafeExternalURL(linkURL) }
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
		}).OnClick(openLink)
		return
	}
	iw, ih := st.bitmap.Size()
	btn := ui.ButtonBase(c).Label(label).FillWidth().Radius(10).Clip()
	btn.Children(func() {
		img := ui.Image(c, st.bitmap).FillWidth().Fit(ui.Contain).Radius(10)
		if iw > 0 && ih > 0 {
			img.AspectRatio(float32(iw) / float32(ih))
		}
	}).OnClick(openLink)
}

func youTubeVideoID(u *url.URL, host string) string {
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	if host == "youtu.be" {
		return strings.TrimPrefix(path, "/")
	}
	if !hostMatches(host, "youtube.com") {
		return ""
	}
	if path == "/watch" {
		return u.Query().Get("v")
	}
	for _, prefix := range []string{"/embed/", "/shorts/"} {
		if id, ok := strings.CutPrefix(path, prefix); ok {
			return id
		}
	}
	return ""
}

// Keep #p at full opacity with no CSS transform (avoiding WKWebView cross-origin
// iframe compositing glitches). Seat .loader above #p only until the player
// signals ready (Vimeo/YouTube postMessage or video loadeddata), then fade out
// and remove .loader with display:none so it never leaves a veil over playback.
const videoPlayerBaseCSS = `html,body{margin:0;padding:0;width:100%;height:100%;background:#0c0e12;overflow:hidden;position:relative;font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text",system-ui,sans-serif}` +
	`.loader{position:absolute;inset:0;z-index:3;display:flex;flex-direction:column;align-items:center;justify-content:center;background:#0c0e12;pointer-events:none;opacity:1;transition:opacity .22s ease}` +
	`.shimmer{position:absolute;inset:0;background:linear-gradient(110deg,transparent 25%,rgba(255,255,255,.04) 45%,rgba(255,255,255,.075) 50%,rgba(255,255,255,.04) 55%,transparent 75%);transform:translateX(-100%);animation:shimmer 1.65s cubic-bezier(.4,0,.2,1) infinite}` +
	`.ring{width:32px;height:32px;border-radius:50%;border:2px solid rgba(255,255,255,.12);border-top-color:rgba(255,255,255,.78);animation:spin .8s linear infinite;position:relative;z-index:1}` +
	`.label{margin-top:12px;font-size:12px;color:rgba(255,255,255,.5);letter-spacing:.01em;position:relative;z-index:1}` +
	`#p{position:relative;z-index:1;border:0;width:100%;height:100%;display:block;background:#0c0e12;outline:none;object-fit:contain}` +
	`body.ready .loader{opacity:0}` +
	`body.done .loader{display:none}` +
	`body.err #p,body.err .shimmer,body.err .ring{display:none}` +
	`@keyframes spin{to{transform:rotate(360deg)}}` +
	`@keyframes shimmer{to{transform:translateX(100%)}}` +
	`@media(prefers-reduced-motion:reduce){.shimmer{animation:none}.loader{transition:none}}`

const videoLoaderHTML = `<div class="loader"><div class="shimmer"></div><div class="ring"></div><div class="label">Loading video…</div></div>`

func buildVideoPlayerHTML(videoURL string) string {
	trimmed := strings.TrimSpace(videoURL)
	u, host := parseVideoURL(trimmed)

	if render.IsDirectVideoURL(trimmed) {
		preconnect := ""
		if u != nil && u.Host != "" {
			preconnect = `<link rel="preconnect" href="https://` + html.EscapeString(u.Host) + `" crossorigin>`
		}
		safeURL := html.EscapeString(trimmed)
		onDone := `if(!document.body.classList.contains('ready')){document.body.classList.add('ready');setTimeout(function(){document.body.classList.add('done')},250)}`
		onErr := `var l=document.querySelector('.label');if(l)l.textContent='Video unavailable';document.body.classList.add('err')`
		return `<!doctype html><html><head><meta charset="utf-8">` + preconnect + `<style>` + videoPlayerBaseCSS + `</style></head><body>` + videoLoaderHTML + `<video id="p" controls preload="auto" playsinline src="` + safeURL + `" onloadeddata="` + onDone + `" oncanplay="` + onDone + `" onplaying="` + onDone + `" onerror="` + onErr + `"></video></body></html>`
	}

	embedURL := trimmed
	preconnect := ""
	waitReadyOrigin := ""
	if u != nil {
		if id := youTubeVideoID(u, host); reYouTubeID.MatchString(id) {
			embedURL = "https://www.youtube-nocookie.com/embed/" + id + "?rel=0&modestbranding=1&enablejsapi=1"
			preconnect = `<link rel="preconnect" href="https://www.youtube-nocookie.com" crossorigin><link rel="preconnect" href="https://i.ytimg.com" crossorigin><link rel="preconnect" href="https://s.ytimg.com" crossorigin>`
			waitReadyOrigin = "https://www.youtube-nocookie.com"
		} else if hostMatches(host, "vimeo.com") {
			if m := reVimeoPath.FindStringSubmatch(u.EscapedPath()); m != nil {
				embedURL = "https://player.vimeo.com/video/" + m[1] + "?dnt=1&title=0&byline=0&portrait=0&badge=0&autopause=0&api=1"
				if h := u.Query().Get("h"); render.IsSafeSlug(h) {
					embedURL = "https://player.vimeo.com/video/" + m[1] + "?h=" + h + "&dnt=1&title=0&byline=0&portrait=0&badge=0&autopause=0&api=1"
				}
				preconnect = `<link rel="preconnect" href="https://player.vimeo.com" crossorigin><link rel="preconnect" href="https://f.vimeocdn.com" crossorigin><link rel="preconnect" href="https://i.vimeocdn.com" crossorigin><link rel="dns-prefetch" href="https://fresnel.vimeocdn.com">`
				waitReadyOrigin = "https://player.vimeo.com"
			}
		}
		if preconnect == "" && render.IsTrustedVideoHost(trimmed) {
			preconnect = `<link rel="preconnect" href="https://` + html.EscapeString(u.Host) + `" crossorigin>`
		}
	}
	safeEmbed := html.EscapeString(embedURL)
	var script string
	if waitReadyOrigin != "" {
		// Vimeo fires "ready" before its HD poster finishes replacing the blurred LQIP,
		// so give Vimeo a brief settle delay after ready while keeping YouTube immediate.
		settleMs := "150"
		if waitReadyOrigin == "https://player.vimeo.com" {
			settleMs = "700"
		}
		script = `<script>(function(){var d=false,s=false,o='` + waitReadyOrigin + `';function hide(){if(d)return;d=true;document.body.classList.add('ready');setTimeout(function(){document.body.classList.add('done')},250)}function settle(){if(s||d)return;s=true;setTimeout(hide,` + settleMs + `)}window.__declicOnLoad=function(){var f=document.getElementById('p');if(f&&f.contentWindow&&o==='https://www.youtube-nocookie.com'){f.contentWindow.postMessage('{"event":"listening"}',o)}setTimeout(hide,2600)};window.addEventListener('message',function(e){if(e.origin!==o)return;try{var m=typeof e.data==='string'?JSON.parse(e.data):e.data;if(m&&(m.event==='ready'||m.event==='onReady'||m.event==='initialDelivery'))settle()}catch(_){}});setTimeout(hide,12000)})()</script>`
	} else {
		script = `<script>(function(){var d=false;function hide(){if(d)return;d=true;document.body.classList.add('ready');setTimeout(function(){document.body.classList.add('done')},250)}window.__declicOnLoad=function(){setTimeout(hide,300)};setTimeout(hide,12000)})()</script>`
	}
	return `<!doctype html><html><head><meta charset="utf-8">` + preconnect + `<style>` + videoPlayerBaseCSS + `</style></head><body>` + videoLoaderHTML + script + `<iframe src="` + safeEmbed + `" id="p" loading="eager" fetchpriority="high" onload="window.__declicOnLoad&&window.__declicOnLoad()" allow="autoplay; fullscreen; picture-in-picture" allowfullscreen referrerpolicy="origin"></iframe></body></html>`
}
