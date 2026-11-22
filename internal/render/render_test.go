package render

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
)

func TestDescriptionBoundary(t *testing.T) {
	got := Description("<script>title</script>", `<p onclick="evil()">Hello <sup>2</sup></p><script>evil()</script><iframe src="https://example.com"></iframe><img src="file:///etc/passwd" onerror="evil()"><img src="https://example.com/sample.png"><table><tr><td>Cell</td></tr></table>`)
	for _, unsafe := range []string{"<script>", "<iframe", "onclick=", "onerror=", "file:///"} {
		if strings.Contains(got, unsafe) {
			t.Fatalf("unsafe markup %s", unsafe)
		}
	}
	if !strings.Contains(got, "<sup>2</sup>") || !strings.Contains(got, "<td>Cell</td>") {
		t.Fatal("lost supported formatting")
	}
}

func TestMathMLAndLatexFallback(t *testing.T) {
	got := Description("math", `<math href="javascript:evil()"><mfrac><mn>1</mn><mn>2</mn></mfrac><annotation-xml encoding="text/html"><img src="https://example.com/unwanted"></annotation-xml></math><p>$x^2$ and $\frac{a+b}{2}$</p>`)
	if !strings.Contains(got, "<mfrac><mn>1</mn><mn>2</mn></mfrac>") || !strings.Contains(got, "<msup><mi>x</mi><mn>2</mn></msup>") || !strings.Contains(got, "<mfrac>") {
		t.Fatalf("lost math or LaTeX->MathML conversion: %s", got)
	}
	for _, bad := range []string{"href=", "annotation-xml", "unwanted", "javascript:"} {
		if strings.Contains(got, bad) {
			t.Fatalf("unsafe math content %s", bad)
		}
	}
}

func TestDescriptionTOCAndVideoFallbackCard(t *testing.T) {
	got := Description("Guide", `<h2>Overview</h2><p>Intro</p><h3>Details</h3><iframe src="https://player.vimeo.com/video/12345"></iframe>`)
	for _, want := range []string{"Contents", `id="sec-1"`, `id="sec-2"`, "video-card", LocalVideoSentinel} {
		if !strings.Contains(got, want) {
			t.Fatalf("description missing %q", want)
		}
	}
	if strings.Contains(got, "<iframe") {
		t.Fatal("iframe must not be embedded")
	}
}

func TestDescriptionCollapsesBlankLinesAndDifferentiatesSections(t *testing.T) {
	raw := `<p>Given an array.</p><p>&nbsp;</p><p></p><p><br><br></p><p><strong>Example 1:</strong></p><pre>Input: nums = [2,7]


Output: [0,1]
</pre><p>&nbsp;</p><p><strong>Constraints:</strong></p><ul><li>1 &lt;= n</li></ul>`
	got := Description("Two Sum", raw)
	if strings.Contains(got, "<p>\u00a0</p>") || strings.Contains(got, "<p></p>") || strings.Contains(got, "<br/><br/>") {
		t.Fatalf("blank spacer markup was not collapsed: %s", got)
	}
	if !strings.Contains(got, `<p class="sec-label"><strong>Example 1:</strong></p>`) {
		t.Fatalf("missing section label styling: %s", got)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Fatalf("excessive newlines inside pre were not collapsed")
	}
}

func TestArticleMarkdownPreprocessStripsTOCAndEmbeds(t *testing.T) {
	src := "[TOC]\n\n## Video Solution\n\n<div>\n    <div class=\"video-container\">\n        <iframe src=\"https://player.vimeo.com/video/567281997\" width=\"640\" height=\"360\"></iframe>\n    </div>\n</div>\n\n<div>&nbsp;</div>\n\n## Solution Article\n\n### Approach 1\n\n<iframe src=\"https://leetcode.com/playground/WTVGRyeD/shared\" width=\"100%\" height=\"327\"></iframe>\n"
	cleaned, refs, videos := PreprocessArticleMarkdown(src)
	if strings.Contains(cleaned, "[TOC]") || strings.Contains(cleaned, "<iframe") || strings.Contains(cleaned, "&nbsp;") {
		t.Fatalf("raw TOC or HTML leaked into cleaned markdown:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "```declic-video\nhttps://player.vimeo.com/video/567281997\n```") {
		t.Fatalf("video marker missing:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "```declic-playground\nleetcode.com WTVGRyeD\n```") {
		t.Fatalf("playground marker missing:\n%s", cleaned)
	}
	if len(refs) != 1 || refs[0].Site != leetcode.COM || refs[0].UUID != "WTVGRyeD" {
		t.Fatalf("unexpected playground refs: %+v", refs)
	}
	if len(videos) != 1 || videos[0] != "https://player.vimeo.com/video/567281997" {
		t.Fatalf("unexpected extracted videos: %+v", videos)
	}
}

func TestRichMediaMathAndMermaid(t *testing.T) {
	md := "Price is $5. Complexity is $O(n^2)$ and $a \\le b$, with \\(x_i\\).\n\n$$\\sum_{i=1}^{n} \\frac{i}{2}$$\n\n<video poster=\"https://assets.leetcode.cn/poster.jpg\"><source src=\"https://assets.leetcode.cn/demo.mp4\" type=\"video/mp4\"></video>\n\n[![紺 かっこいい プログラミングの勉強方法 note記事見出し画像 (1).png](https://assets.leetcode.com/users/images/thumb.png)](https://youtu.be/abc123XYZ)\n\n[![Diagram](https://assets.leetcode.com/users/images/diag.png)](https://example.com/guide)\n\n!?!../Documents/1_fix_1.json:1000,562!?!\n"
	cleaned, _, videos := PreprocessArticleMarkdown(md)
	if !strings.Contains(cleaned, "Price is $5.") || !strings.Contains(cleaned, "$O(n^2)$") {
		t.Fatalf("inline math or currency unexpectedly altered:\n%s", cleaned)
	}
	if obj, ok := InlineMathObject(`\textit{nums}[n - 1] - \textit{nums}[0]`, 14); !ok || obj.Width <= 0 || obj.Height <= 0 {
		t.Fatalf("InlineMathObject failed: %+v %v", obj, ok)
	}
	var italicSpans []ui.Span
	appendInlineMathSpans(&italicSpans, parseTeX(`\textit{nums}[0]`), 14)
	if len(italicSpans) < 2 || italicSpans[0].Text != "nums" || !italicSpans[0].Italic || italicSpans[1].Italic {
		t.Fatalf("\\textit{nums}[0] should render italic 'nums' and upright '[0]', got %+v", italicSpans)
	}
	if obj, ok := InlineMathObject("0", 14); !ok || obj.Width <= 0 {
		t.Fatalf("single-digit math $0$ should render as serif inline math: %+v %v", obj, ok)
	}
	if got, _, _ := PreprocessArticleMarkdown("Always at least $0$ and $1 \\le n \\le 10^4$, costs $5."); !strings.Contains(got, `\(0\)`) || !strings.Contains(got, `\(1 \le n \le 10^4\)`) || !strings.Contains(got, "costs $5.") {
		t.Fatalf("digit-started inline math handling failed: %s", got)
	}
	escaped := `# Code\n` + "```java []" + `\nclass Solution {\n    public String f() {\n        return "a\\nb";\n    }\n}\n` + "```"
	unescaped, _, _ := PreprocessArticleMarkdown(escaped)
	if strings.Contains(unescaped, `\nclass`) || strings.Contains(unescaped, "java []") {
		t.Fatalf("escaped UGC markdown was not restored:\n%s", unescaped)
	}
	if !strings.Contains(unescaped, "# Code\n```java\nclass Solution {") || !strings.Contains(unescaped, `return "a\nb";`) {
		t.Fatalf("unexpected unescaped UGC markdown:\n%s", unescaped)
	}
	if !strings.Contains(cleaned, "$$\\sum_{i=1}^{n} \\frac{i}{2}$$") {
		t.Fatalf("display math block should remain for Options.Math:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "```declic-video\nhttps://assets.leetcode.cn/demo.mp4\n```") {
		t.Fatalf("HTML5 video tag not extracted:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "```declic-video\nhttps://youtu.be/abc123XYZ\n```") {
		t.Fatalf("linked YouTube video thumbnail not extracted:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "```declic-linked-image\nhttps://example.com/guide\nhttps://assets.leetcode.com/users/images/diag.png\nDiagram\n```") {
		t.Fatalf("linked image not extracted:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "```declic-slides\n../Documents/1_fix_1.json:1000,562\n```") {
		t.Fatalf("slides spec not preserved:\n%s", cleaned)
	}
	if len(videos) != 2 {
		t.Fatalf("expected 2 pre-warmable videos, got %+v", videos)
	}

	multi := "## Code\n\n**C++**\n```cpp\nint a = 1;\n```\n\n**Python3**\n```python3\na = 1\n```\n\n```go\na := 1\n```\n"
	merged, _, _ := PreprocessArticleMarkdown(multi)
	if !strings.Contains(merged, "```declic-codetabs\n") || !strings.Contains(merged, `"langSlug":"cpp"`) || !strings.Contains(merged, `"langSlug":"python3"`) || !strings.Contains(merged, `"langSlug":"golang"`) {
		t.Fatalf("multi-language code blocks did not merge:\n%s", merged)
	}

	fc := ParseMermaid("flowchart LR\n  A[Input] --> B{Check}\n  B -->|Yes| C((Done))\n  B -->|No| D(Retry)")
	if fc.Kind != MermaidFlowchart || len(fc.Nodes) != 4 || len(fc.Edges) != 3 {
		t.Fatalf("mermaid flowchart parse failed: %+v", fc)
	}
	seq := ParseMermaid("sequenceDiagram\n  participant A as Alice\n  A->>Bob: Hello\n  Bob-->>A: Hi")
	if seq.Kind != MermaidSequence || len(seq.Actors) != 2 || len(seq.SeqSteps) != 2 || seq.NodeByID["A"].Label != "Alice" {
		t.Fatalf("mermaid sequence parse failed: %+v", seq)
	}
	paidVidMD, _, _ := PreprocessArticleMarkdown("## Video Solution\n---\n\n<div>&nbsp;</div>\n\n## Solution Article\n\n### Approach 1\nBody")
	if strings.Contains(paidVidMD, "Video Solution") || !strings.Contains(paidVidMD, "## Solution Article") {
		t.Fatalf("expected empty Video Solution section to be hidden:\n%s", paidVidMD)
	}
	cnVidMD, _, _ := PreprocessArticleMarkdown("### 📺 视频题解\n![1.两数之和.mp4](4547de8a-5963-4caf-9281-c22ee751ab12)\n")
	if !strings.Contains(cnVidMD, "```declic-cn-video\n4547de8a-5963-4caf-9281-c22ee751ab12\n```") {
		t.Fatalf("expected leetcode.cn UUID video fence:\n%s", cnVidMD)
	}
}

func TestRenderSecurityAndEdgeCases(t *testing.T) {
	if !IsDirectVideoURL("https://cdn.example/demo.mp4") || IsDirectVideoURL("https:///demo.mp4") || IsDirectVideoURL("https://u:p@cdn.example/demo.mp4") {
		t.Fatal("IsDirectVideoURL must require HTTPS host without userinfo")
	}
	if slug, ok := ParseLocalProblemURL("https://declic.local/problem/3sum"); !ok || slug != "3sum" {
		t.Fatalf("ParseLocalProblemURL valid slug failed: %q %v", slug, ok)
	}
	if _, ok := ParseLocalProblemURL("https://declic.local/problem/../etc"); ok {
		t.Fatal("ParseLocalProblemURL accepted unsafe slug")
	}
	desc := Description("", `<p>Body</p><noscript>secret-noscript</noscript><img src="file:///etc/passwd">`)
	for _, bad := range []string{"<h1>", "secret-noscript", "<img"} {
		if strings.Contains(desc, bad) {
			t.Fatalf("Description leaked %q: %s", bad, desc)
		}
	}
	md, _, _ := PreprocessArticleMarkdown(`<a href="javascript:alert(1)"><img data-src="https://evil.com/x.png" src="https://assets.leetcode.com/ok.png"></a>` + "\n!?!bad```fence.json!?!\n")
	if strings.Contains(md, "javascript:") || strings.Contains(md, "evil.com") || strings.Contains(md, "declic-slides") {
		t.Fatalf("unsafe markdown leaked:\n%s", md)
	}
	if got := mathNodeUnicode(parseTeX(`\displaystyle .5 + α`)); !strings.Contains(got, ".5") || !strings.Contains(got, "α") {
		t.Fatalf("TeX parser corrupted decimal or UTF-8 rune: %q", got)
	}
}
