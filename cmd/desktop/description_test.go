package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/render"
)

func TestHintsSeparateAndSimilarQuestionsMetadata(t *testing.T) {
	a := &app{
		site:            leetcode.COM,
		descriptionSize: 15,
		catalog: []leetcode.Problem{
			{FrontendID: "15", Slug: "3sum", Title: "3Sum", Difficulty: "Medium", Paid: false},
			{FrontendID: "170", Slug: "two-sum-iii", Title: "Two Sum III", Difficulty: "Easy", Paid: true},
		},
		problem: leetcode.Problem{
			Hints:   []string{"Try a hash map.", "Watch out for <code>duplicates</code>."},
			Similar: `[{"title":"3Sum","titleSlug":"3sum","difficulty":"Medium","isPaidOnly":false},{"title":"Two Sum III","titleSlug":"two-sum-iii","difficulty":"Easy","isPaidOnly":true}]`,
		},
	}
	got := a.statementHTML("<p>Statement</p>")
	for _, want := range []string{
		"Hint 1</span></summary>",
		"Hint 2</span></summary>",
		"<code>duplicates</code>",
		"Similar Questions (2)",
		`href="https://declic.local/problem/3sum"`,
		">15.<",
		">170.<",
		"Premium",
		"Medium",
		"Easy",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("statementHTML missing %q", want)
		}
	}
}

func TestSlidesLoginURLAndJudgeDiffHelpers(t *testing.T) {
	urls := resolveSlideManifestURLs(leetcode.COM, "two-sum", "../Documents/1_fix_1.json:1000,562")
	if len(urls) == 0 || !strings.HasSuffix(urls[0], "/Documents/1_fix_1.json") {
		t.Fatalf("slide manifest URL resolution failed: %+v", urls)
	}
	for _, bad := range []string{"//evil.com/manifest.json", "HTTPS://evil.com/manifest.json", "http://leetcode.com/manifest.json"} {
		if got := resolveSlideManifestURLs(leetcode.COM, "two-sum", bad); len(got) != 0 {
			t.Fatalf("resolveSlideManifestURLs accepted untrusted spec %q: %+v", bad, got)
		}
	}
	if !isPostLoginSiteURL(leetcode.COM, "https://leetcode.com/problemset/?next=/accounts/login") ||
		isPostLoginSiteURL(leetcode.COM, "https://leetcode.com.evil.com/problemset/") ||
		isPostLoginSiteURL(leetcode.COM, "https://leetcode.com/accounts/login/") {
		t.Fatal("isPostLoginSiteURL host/path validation failed")
	}
	frames := parseSlideManifestFrames("https://leetcode.com/problems/two-sum/Documents/1_fix_1.json", []byte(`{"timeline":[{"image":"../Figures/1/1_1.png"},{"image":"../Figures/1/1_2.png"}]}`))
	if len(frames) != 2 || frames[0] != "https://leetcode.com/problems/two-sum/Figures/1/1_1.png" {
		t.Fatalf("slide frame parsing failed: %+v", frames)
	}

	p, ma, mb, s := diffCommonPrefixSuffix("[0,1,2,3]", "[0,1,9,3]")
	if p != "[0,1," || ma != "2" || mb != "9" || s != ",3]" {
		t.Fatalf("diffCommonPrefixSuffix mismatch: %q %q %q %q", p, ma, mb, s)
	}
	if _, trunc := truncateResultText(strings.Repeat("x", 300)); !trunc {
		t.Fatal("long result text should truncate")
	}
}

func TestMarkdownCJKAndDefaultExtensionsRender(t *testing.T) {
	a := &app{site: leetcode.CN}
	raw := strings.Join([]string{
		"请看 https://example.com/guide，然后继续阅读。",
		"",
		"**结论。**后文提到 @author 和 #123，以及至少 $0$ 到 $1 \\le n \\le 10^4$，预算 $5。",
		"",
		"> [!NOTE]",
		"> 关键提示内容。",
	}, "\n")
	cleaned, _, _ := render.PreprocessArticleMarkdown(raw)
	baseOpts := *a.markdownOptions()
	var clickedURL string
	var mathInputs []string
	baseOpts.OnLink = func(u string) { clickedURL = u }
	origInlineMath := baseOpts.InlineMath
	baseOpts.InlineMath = func(src string, size float32) (markdown.InlineObject, bool) {
		mathInputs = append(mathInputs, src)
		return origInlineMath(src, size)
	}
	v := ui.NewTester(func(c *ui.Context) {
		a.applyTheme(c)
		ui.Column(c).Fill().Children(func() {
			markdown.View(c, cleaned, &baseOpts).FillWidth()
		})
	}, 720, 600)
	v.Frame()

	// Verify CJK autolink ends before Chinese punctuation and routes the clean URL on click.
	if err := v.Click("https://example.com/guide"); err != nil {
		t.Fatalf("failed to click CJK-bounded autolink: %v", err)
	}
	if clickedURL != "https://example.com/guide" {
		t.Fatalf("autolink click target mismatch: got %q", clickedURL)
	}
	if _, ok := v.Find("https://example.com/guide，然后继续阅读。"); ok {
		t.Fatal("autolink unexpectedly swallowed trailing CJK punctuation")
	}
	// Verify digit-started inline math reached InlineMath without misparsing currency $5.
	wantMath := map[string]bool{"0": false, `1 \le n \le 10^4`: false}
	for _, m := range mathInputs {
		if m == "5" {
			t.Fatal("currency $5 was mistakenly parsed as inline math")
		}
		if _, ok := wantMath[m]; ok {
			wantMath[m] = true
		}
	}
	for src, seen := range wantMath {
		if !seen {
			t.Fatalf("expected InlineMath to receive %q, got %v", src, mathInputs)
		}
	}
	// Verify CJK bold did not leak raw asterisks and unresolved mention/reference/currency remained text.
	if _, ok := v.Find("**结论。**"); ok {
		t.Fatal("CJK emphasis leaked raw ** markers")
	}
	// Verify GitHub alert rendered both its Note heading and body.
	if _, ok := v.Find("Note"); !ok {
		t.Fatal("expected GitHub alert title 'Note' to render")
	}
	if _, ok := v.Find("关键提示内容。"); !ok {
		t.Fatal("expected GitHub alert body to render")
	}
}
