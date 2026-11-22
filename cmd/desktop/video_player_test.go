package main

import (
	"html"
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/render"
)

func TestVideoPlayerSource(t *testing.T) {
	const youtube = "https://www.youtube-nocookie.com/embed/abc123XYZ?rel=0&modestbranding=1&enablejsapi=1"
	const vimeo = "https://player.vimeo.com/video/12345?dnt=1&title=0&byline=0&portrait=0&badge=0&autopause=0&api=1"
	const vimeoHash = "https://player.vimeo.com/video/12345?h=deadbeef01&dnt=1&title=0&byline=0&portrait=0&badge=0&autopause=0&api=1"
	for _, tt := range []struct{ input, want string }{
		{"https://www.YouTube.com/watch?list=demo&v=abc123XYZ", youtube},
		{"https://youtube.com/embed/abc123XYZ", youtube},
		{"https://m.youtube.com/shorts/abc123XYZ/", youtube},
		{"//youtu.be/abc123XYZ?t=15", youtube},
		{"https://vimeo.com/12345", vimeo},
		{"https://PLAYER.vimeo.com/video/12345/", vimeo},
		{"https://player.vimeo.com/video/12345?h=deadbeef01", vimeoHash},
		// An empty want means the iframe should retain the original URL.
		{"https://leetcode.com/videos/vimeo.com/12345", ""},
		{"https://player.vimeo.com/show?related=youtu.be/abc123XYZ", ""},
		{"https://evil-vimeo.com/12345", ""},
		{"https://vimeo.com.evil.com/12345", ""},
		{"http://youtube.com/watch?v=abc123XYZ", ""},
		{"https://youtu.be/abc123XYZ/extra", ""},
		{"https://youtube.com/watch?v=abc123XYZ%21", ""},
		{"https://vimeo.com/12345extra", ""},
		{"https://www.youtube-nocookie.com/embed/abc123XYZ?start=30", ""},
	} {
		t.Run(tt.input, func(t *testing.T) {
			want := tt.want
			if want == "" {
				want = tt.input
			}
			got := buildVideoPlayerHTML(tt.input)
			src := `<iframe src="` + html.EscapeString(want) + `"`
			if !strings.Contains(got, src) {
				t.Errorf("expected %s in player HTML, got %s", src, got)
			}
			if !strings.Contains(got, `class="loader"`) || !strings.Contains(got, `class="shimmer"`) || !strings.Contains(got, `body.done .loader{display:none}`) || strings.Contains(got, `#p{position:relative;z-index:1;border:0;width:100%;height:100%;display:block;background:#0c0e12;outline:none;object-fit:contain;opacity:0`) {
				t.Errorf("player HTML missing clean loader fade-out or applied opacity to #p: %s", got)
			}
			if hasPreconnect := strings.Contains(got, `rel="preconnect"`); hasPreconnect != render.IsTrustedVideoHost(tt.input) {
				t.Errorf("preconnect=%v, want %v in %s", hasPreconnect, render.IsTrustedVideoHost(tt.input), got)
			}
		})
	}
	direct := buildVideoPlayerHTML("https://cdn.example/demo.mp4")
	if !strings.Contains(direct, `preload="auto"`) || !strings.Contains(direct, `class="loader"`) || !strings.Contains(direct, `onloadeddata`) {
		t.Fatalf("direct video player missing eager preload or loading state: %s", direct)
	}
}

func TestVideoCardRendersPure16By9Window(t *testing.T) {
	a := &app{
		site:    leetcode.COM,
		problem: leetcode.Problem{Slug: "two-sum", Title: "Two Sum"},
	}
	v := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(640).Children(func() {
			a.renderVideoSolutionCard(c, "https://player.vimeo.com/video/12345")
			a.renderVideoSolutionCard(c, "file:///etc/passwd")
			ui.Text(c, "after-video")
		})
	}, 640, 480)
	v.Frame()
	if v.HasText("Two Sum — Video Solution") || v.HasText("Video Solution") {
		t.Fatal("video window should not render a title footer")
	}
	if _, ok := v.Find("Open in browser"); ok {
		t.Fatal("video window should not render an Open in browser button")
	}
	if !v.HasText("Loading video…") || v.HasText("Vimeo") {
		t.Fatal("expected generic Loading video… text without provider name")
	}
	// Verify only the trusted video card rendered at exact 16:9 (640 x 360) with no extra bar.
	after, ok := v.Find("after-video")
	if !ok || math.Abs(float64(after.Y-360)) > 1 {
		t.Fatalf("expected 16:9 video height ~360 at width 640, got Y=%.1f (ok=%v)", after.Y, ok)
	}
}

func TestCNWorkspaceHidesEditorialAndRendersCNVideoPlayer(t *testing.T) {
	a, _ := testWorkspace(t)
	a.site = leetcode.CN
	a.resourceTab = "Editorial"
	v := ui.NewTester(a.view, 1360, 860)
	v.Frame()
	if _, ok := v.Find("Editorial"); ok {
		t.Fatal("leetcode.cn workspace should hide Editorial tab")
	}
	if a.resourceTab != "Description" {
		t.Fatalf("leetcode.cn should fall back from Editorial to Description, got %q", a.resourceTab)
	}

	info := leetcode.CNVideoInfo{
		VideoID:  "vid123",
		PlayAuth: "tok</script>abc",
		CoverURL: "https://video.leetcode.cn/cover.png",
		CanSee:   true,
	}
	prewarm := buildCNVideoPlayerHTML(info, false)
	if !strings.Contains(prewarm, "aliplayer-min.js") || !strings.Contains(prewarm, "encryptType:1") || strings.Contains(prewarm, "tok</script>abc") {
		t.Fatalf("unexpected CN video player HTML: %s", prewarm)
	}
	if !strings.Contains(prewarm, `"autoplay":false`) || !strings.Contains(prewarm, "window.__declicPlay") || !strings.Contains(prewarm, "declic:playing") || !strings.Contains(prewarm, ".prism-controlbar-bg{background:rgba(20,22,26,.88)!important") {
		t.Fatalf("prewarm CN video HTML missing autoplay=false, playback hooks, or dark controlbar contrast: %s", prewarm)
	}
	auto := buildCNVideoPlayerHTML(info, true)
	if !strings.Contains(auto, `"autoplay":true`) {
		t.Fatalf("autoplay CN video HTML missing autoplay=true: %s", auto)
	}
}
