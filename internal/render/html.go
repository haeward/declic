package render

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"strings"

	xhtml "golang.org/x/net/html"
)

const (
	LocalProblemPrefix = "https://declic.local/problem/"
	LocalVideoSentinel = "https://declic.local/video"
)

// IsSafeSlug checks that a problem slug or playground UUID contains only safe identifier characters.
func IsSafeSlug(slug string) bool {
	if slug == "" || len(slug) > 160 {
		return false
	}
	for _, r := range slug {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// ParseLocalProblemURL extracts a safe problem slug from a declic.local problem link.
func ParseLocalProblemURL(raw string) (string, bool) {
	slug, ok := strings.CutPrefix(raw, LocalProblemPrefix)
	if !ok || !IsSafeSlug(slug) {
		return "", false
	}
	return slug, true
}

// IsDirectVideoURL reports whether raw is an HTTPS direct video stream URL.
func IsDirectVideoURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return false
	}
	low := strings.ToLower(u.Path)
	return strings.HasSuffix(low, ".mp4") || strings.HasSuffix(low, ".webm") || strings.HasSuffix(low, ".mov") || strings.HasSuffix(low, ".m4v")
}

// IsTrustedVideoHost reports whether raw points to an allowlisted HTTPS video host.
func IsTrustedVideoHost(raw string) bool {
	cleaned := strings.TrimSpace(raw)
	if strings.HasPrefix(cleaned, "//") {
		cleaned = "https:" + cleaned
	}
	u, err := url.Parse(cleaned)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range []string{
		"youtube.com", "youtu.be", "youtube-nocookie.com",
		"vimeo.com", "bilibili.com", "wistia.net", "wistia.com",
		"loom.com", "mux.com", "v.qq.com", "youku.com",
		"leetcode.com", "leetcode.cn", "alicdn.com", "aliyuncs.com",
	} {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

type tocHeading struct {
	ID    string
	Level string
	Text  string
}

func extractNodeText(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(cur *xhtml.Node) {
		if cur.Type == xhtml.TextNode {
			b.WriteString(cur.Data)
		}
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

func hasVisualContent(n *xhtml.Node) bool {
	if n.Type == xhtml.TextNode {
		cleaned := strings.ReplaceAll(n.Data, "\u00a0", "")
		return strings.TrimSpace(cleaned) != ""
	}
	if n.Type == xhtml.ElementNode {
		switch n.Data {
		case "img", "math", "table", "pre", "hr", "a", "ul", "ol", "blockquote":
			return true
		}
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if hasVisualContent(ch) {
			return true
		}
	}
	return false
}

func isSectionHeadingParagraph(p *xhtml.Node) bool {
	if p.Type != xhtml.ElementNode || p.Data != "p" {
		return false
	}
	var strong *xhtml.Node
	for ch := p.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == xhtml.TextNode {
			cleaned := strings.ReplaceAll(ch.Data, "\u00a0", "")
			if strings.TrimSpace(cleaned) != "" {
				return false
			}
			continue
		}
		if ch.Type == xhtml.ElementNode && (ch.Data == "strong" || ch.Data == "b") && strong == nil {
			strong = ch
			continue
		}
		return false
	}
	if strong == nil {
		return false
	}
	txt := strings.TrimSpace(strings.ReplaceAll(extractNodeText(strong), "\u00a0", " "))
	return txt != "" && len([]rune(txt)) <= 64
}

func collapsePreNewlines(n *xhtml.Node) {
	var collect func(*xhtml.Node)
	var nodes []*xhtml.Node
	collect = func(cur *xhtml.Node) {
		if cur.Type == xhtml.TextNode {
			nodes = append(nodes, cur)
		}
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			collect(ch)
		}
	}
	collect(n)
	for i, t := range nodes {
		s := strings.ReplaceAll(t.Data, "\r\n", "\n")
		for strings.Contains(s, "\n\n\n") {
			s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
		}
		if i == 0 {
			s = strings.TrimLeft(s, "\n")
		}
		if i == len(nodes)-1 {
			s = strings.TrimRight(s, "\n")
		}
		t.Data = s
	}
}

func cleanupSanitizedTree(node *xhtml.Node, inPre bool) {
	if node.Type == xhtml.ElementNode && node.Data == "pre" {
		collapsePreNewlines(node)
		return
	}
	var prevWasBR bool
	for ch := node.FirstChild; ch != nil; {
		next := ch.NextSibling
		if ch.Type == xhtml.ElementNode {
			cleanupSanitizedTree(ch, inPre || ch.Data == "pre")
			if (ch.Data == "p" || ch.Data == "div") && !hasVisualContent(ch) {
				node.RemoveChild(ch)
				ch = next
				continue
			}
			if ch.Data == "p" && isSectionHeadingParagraph(ch) {
				ch.Attr = append(ch.Attr, xhtml.Attribute{Key: "class", Val: "sec-label"})
			}
			if !inPre && ch.Data == "br" {
				if prevWasBR {
					node.RemoveChild(ch)
					ch = next
					continue
				}
				prevWasBR = true
			} else {
				prevWasBR = false
			}
		} else if ch.Type == xhtml.TextNode {
			cleaned := strings.ReplaceAll(ch.Data, "\u00a0", "")
			if strings.TrimSpace(cleaned) != "" {
				prevWasBR = false
			}
		}
		ch = next
	}
	if !inPre && node.Type == xhtml.ElementNode && (node.Data == "p" || node.Data == "div" || node.Data == "li") {
		for node.FirstChild != nil {
			f := node.FirstChild
			if f.Type == xhtml.ElementNode && f.Data == "br" {
				node.RemoveChild(f)
				continue
			}
			if f.Type == xhtml.TextNode && strings.TrimSpace(strings.ReplaceAll(f.Data, "\u00a0", "")) == "" {
				node.RemoveChild(f)
				continue
			}
			break
		}
		for node.LastChild != nil {
			l := node.LastChild
			if l.Type == xhtml.ElementNode && l.Data == "br" {
				node.RemoveChild(l)
				continue
			}
			if l.Type == xhtml.TextNode && strings.TrimSpace(strings.ReplaceAll(l.Data, "\u00a0", "")) == "" {
				node.RemoveChild(l)
				continue
			}
			break
		}
	}
}

func appendTextWithMathML(parent *xhtml.Node, text string, inCodeOrMath bool) {
	if inCodeOrMath || (!strings.Contains(text, "$") && !strings.Contains(text, `\(`) && !strings.Contains(text, `\[`)) {
		parent.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: text})
		return
	}
	appendMath := func(tex string, display bool) {
		parent.AppendChild(texToMathMLNode(tex, display))
	}
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			parent.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: buf.String()})
			buf.Reset()
		}
	}
	for i := 0; i < len(text); {
		if text[i] == '$' && i+1 < len(text) && text[i+1] == '$' {
			if end := strings.Index(text[i+2:], "$$"); end > 0 {
				flush()
				appendMath(text[i+2:i+2+end], true)
				i += 2 + end + 2
				continue
			}
		}
		if text[i] == '\\' && i+1 < len(text) && (text[i+1] == '[' || text[i+1] == '(') {
			closeSeq := `\)`
			display := false
			if text[i+1] == '[' {
				closeSeq = `\]`
				display = true
			}
			if end := strings.Index(text[i+2:], closeSeq); end > 0 {
				flush()
				appendMath(text[i+2:i+2+end], display)
				i += 2 + end + 2
				continue
			}
		}
		if text[i] == '$' && (i == 0 || text[i-1] != '\\') && i+1 < len(text) && text[i+1] != ' ' && text[i+1] != '\t' && text[i+1] != '\n' {
			j := i + 1
			found := -1
			for j < len(text) {
				if text[j] == '\n' {
					break
				}
				if text[j] == '$' && text[j-1] != '\\' {
					if text[j-1] != ' ' && text[j-1] != '\t' && (j+1 >= len(text) || text[j+1] < '0' || text[j+1] > '9') {
						found = j
					}
					break
				}
				j++
			}
			if found > i+1 && !isPlainPrice(text[i+1:found]) {
				flush()
				appendMath(text[i+1:found], false)
				i = found + 1
				continue
			}
		}
		buf.WriteByte(text[i])
		i++
	}
	flush()
}

func sanitizeHTMLWithTOC(source string) (string, []tocHeading) {
	doc, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		return html.EscapeString(source), nil
	}
	allowed := map[string]bool{"p": true, "div": true, "span": true, "strong": true, "b": true, "em": true, "i": true, "pre": true, "code": true, "ul": true, "ol": true, "li": true, "br": true, "hr": true, "sub": true, "sup": true, "table": true, "thead": true, "tbody": true, "tr": true, "th": true, "td": true, "h1": true, "h2": true, "h3": true, "h4": true, "blockquote": true, "img": true}
	// Presentation MathML only; no annotation XML, foreign HTML or links.
	for _, tag := range []string{"math", "mrow", "mi", "mn", "mo", "mfrac", "msup", "msub", "msubsup", "msqrt", "mroot", "mover", "munder", "munderover", "mtable", "mtr", "mtd", "mtext"} {
		allowed[tag] = true
	}
	root := &xhtml.Node{Type: xhtml.ElementNode, Data: "div"}
	var headings []tocHeading
	var copyNode func(*xhtml.Node, *xhtml.Node, bool)
	copyNode = func(parent, node *xhtml.Node, inCodeOrMath bool) {
		if node.Type == xhtml.TextNode {
			appendTextWithMathML(parent, node.Data, inCodeOrMath)
			return
		}
		if node.Data == "iframe" || node.Data == "video" {
			src := ""
			for _, attr := range node.Attr {
				if attr.Key == "src" && attr.Val != "" {
					src = attr.Val
					break
				}
			}
			if src == "" && node.Data == "video" {
				for ch := node.FirstChild; ch != nil; ch = ch.NextSibling {
					if ch.Type == xhtml.ElementNode && ch.Data == "source" {
						for _, attr := range ch.Attr {
							if attr.Key == "src" && attr.Val != "" {
								src = attr.Val
								break
							}
						}
					}
					if src != "" {
						break
					}
				}
			}
			if IsTrustedVideoHost(src) {
				card := &xhtml.Node{Type: xhtml.ElementNode, Data: "a", Attr: []xhtml.Attribute{
					{Key: "class", Val: "video-card"},
					{Key: "href", Val: LocalVideoSentinel},
				}}
				card.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: "▶ Watch video walkthrough on LeetCode ↗"})
				parent.AppendChild(card)
			}
			return
		}
		switch node.Data {
		case "script", "style", "noscript", "template", "svg", "title", "textarea", "object", "embed", "noembed", "noframes", "annotation-xml", "annotation":
			return
		}
		nextInCodeOrMath := inCodeOrMath
		if node.Type == xhtml.ElementNode && allowed[node.Data] {
			if node.Data == "pre" || node.Data == "code" || node.Data == "math" {
				nextInCodeOrMath = true
			}
			clean := &xhtml.Node{Type: xhtml.ElementNode, Data: node.Data}
			if node.Data == "img" {
				hasSrc := false
				for _, attr := range node.Attr {
					if attr.Key == "src" {
						u, err := url.Parse(attr.Val)
						if err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil {
							clean.Attr = append(clean.Attr, attr)
							hasSrc = true
						}
					}
					if attr.Key == "alt" && len(attr.Val) <= 200 {
						clean.Attr = append(clean.Attr, attr)
					}
				}
				if !hasSrc {
					return
				}
			}
			if node.Data == "h2" || node.Data == "h3" {
				if txt := extractNodeText(node); txt != "" {
					id := fmt.Sprintf("sec-%d", len(headings)+1)
					clean.Attr = append(clean.Attr, xhtml.Attribute{Key: "id", Val: id})
					headings = append(headings, tocHeading{ID: id, Level: node.Data, Text: txt})
				}
			}
			parent.AppendChild(clean)
			parent = clean
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			copyNode(parent, child, nextInCodeOrMath)
		}
	}
	copyNode(root, doc, false)
	cleanupSanitizedTree(root, false)
	var body bytes.Buffer
	_ = xhtml.Render(&body, root)
	return body.String(), headings
}

// ExtractHTMLText strips raw HTML tags and returns normalized plain text.
func ExtractHTMLText(rawHTML string) string {
	trimmed := strings.TrimSpace(rawHTML)
	if trimmed == "" {
		return ""
	}
	doc, err := xhtml.Parse(strings.NewReader(trimmed))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(extractNodeText(doc), "\u00a0", " "))
}

// SanitizeHTMLFragment sanitizes untrusted HTML and converts inline/display LaTeX to MathML.
func SanitizeHTMLFragment(source string) string {
	out, _ := sanitizeHTMLWithTOC(source)
	return out
}

// Description builds a complete sanitized HTML document with default System appearance.
// External images are HTTPS-only; CSP also blocks scripts, frames, forms and connections.
func Description(title, source string) string {
	return DescriptionWithAppearance(title, source, "System")
}

// DescriptionWithAppearance builds a complete sanitized HTML document styled for the given appearance.
func DescriptionWithAppearance(title, source, appearance string) string {
	body, headings := sanitizeHTMLWithTOC(source)
	var toc strings.Builder
	if len(headings) >= 2 {
		toc.WriteString(`<details class="toc"><summary><span class="sum-left"><svg class="sum-ic" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M5 4h8M5 8h8M5 12h6M2.5 4h.01M2.5 8h.01M2.5 12h.01"/></svg>Contents</span></summary><div class="toc-list">`)
		for _, h := range headings {
			cls := "toc-item"
			if h.Level == "h3" {
				cls = "toc-item toc-sub"
			}
			fmt.Fprintf(&toc, `<a class="%s" href="about:blank#%s">%s</a>`, cls, html.EscapeString(h.ID), html.EscapeString(h.Text))
		}
		toc.WriteString(`</div></details>`)
	}
	theme := "system"
	if appearance == "Dark" {
		theme = "dark"
	}
	if appearance == "Light" {
		theme = "light"
	}
	heading := ""
	if title != "" {
		heading = `<h1>` + html.EscapeString(title) + `</h1>`
	}
	return `<!doctype html><html data-theme="` + theme + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src https:; style-src 'unsafe-inline'"><style>
 :root{color-scheme:light;--bg:#ffffff;--text:#1d1d1f;--title:#1d1d1f;--muted:#6b6b73;--line:#ececf1;--code:#f5f5f8;--hover:#f6f7fb;--accent:#0a4fc0;--easy-fg:#0b7a6b;--easy-bg:#e0f5f0;--med-fg:#8a5200;--med-bg:#fff0d2;--hard-fg:#b3261e;--hard-bg:#fde8e8;--pm-fg:#5b3fd0;--pm-bg:#efeaff;--img-bg:transparent;--img-pad:0}
 @media(prefers-color-scheme:dark){:root{color-scheme:dark;--bg:#1a1b1f;--text:#c9cad0;--title:#e9e9ec;--muted:#8c8d95;--line:#262830;--code:#141519;--hover:#22242a;--accent:#6ea6ff;--easy-fg:#4cc38a;--easy-bg:#12382d;--med-fg:#f5b942;--med-bg:#3a2b10;--hard-fg:#ff7a70;--hard-bg:#3d1c1d;--pm-fg:#b89dff;--pm-bg:#2a2244;--img-bg:#f4f5f8;--img-pad:10px}}
 :root[data-theme=dark]{color-scheme:dark;--bg:#1a1b1f;--text:#c9cad0;--title:#e9e9ec;--muted:#8c8d95;--line:#262830;--code:#141519;--hover:#22242a;--accent:#6ea6ff;--easy-fg:#4cc38a;--easy-bg:#12382d;--med-fg:#f5b942;--med-bg:#3a2b10;--hard-fg:#ff7a70;--hard-bg:#3d1c1d;--pm-fg:#b89dff;--pm-bg:#2a2244;--img-bg:#f4f5f8;--img-pad:10px}
 :root[data-theme=light]{color-scheme:light;--bg:#ffffff;--text:#1d1d1f;--title:#1d1d1f;--muted:#6b6b73;--line:#ececf1;--code:#f5f5f8;--hover:#f6f7fb;--accent:#0a4fc0;--easy-fg:#0b7a6b;--easy-bg:#e0f5f0;--med-fg:#8a5200;--med-bg:#fff0d2;--hard-fg:#b3261e;--hard-bg:#fde8e8;--pm-fg:#5b3fd0;--pm-bg:#efeaff;--img-bg:transparent;--img-pad:0}
 *{box-sizing:border-box}html{scroll-behavior:smooth}html,body{scrollbar-width:none}::-webkit-scrollbar{display:none}
 body{margin:0;padding:8px 22px 28px;font:14px/1.66 -apple-system,BlinkMacSystemFont,"SF Pro Text",system-ui,sans-serif;background:var(--bg);color:var(--text);overflow-wrap:anywhere}
 h1{font-size:1.48em;font-weight:700;line-height:1.28;color:var(--title);margin:0 0 14px;letter-spacing:-.3px}
 h2{font-size:1.24em;font-weight:700;line-height:1.34;color:var(--title);margin:22px 0 8px;letter-spacing:-.15px}
 h3{font-size:1.10em;font-weight:600;line-height:1.38;color:var(--title);margin:18px 0 6px}
 h4{font-size:1.02em;font-weight:600;line-height:1.4;color:var(--title);margin:14px 0 4px}
 p{margin:0 0 10px}
 p.sec-label{font-size:1.07em;font-weight:650;line-height:1.4;color:var(--title);margin:18px 0 6px}
 p.sec-label+pre,p.sec-label+ul,p.sec-label+ol{margin-top:6px}
 pre{white-space:pre-wrap;overflow-x:auto;background:var(--code);border:1px solid var(--line);border-radius:10px;padding:12px 14px;margin:10px 0 14px;font:12.5px/1.62 ui-monospace,SFMono-Regular,Menlo,monospace}
 blockquote{background:var(--code);border:1px solid var(--line);border-radius:10px;padding:10px 14px;margin:12px 0;color:var(--muted)}
 pre strong{color:var(--title);font-weight:600}
 code{font:12.5px/1.6 ui-monospace,SFMono-Regular,Menlo,monospace;background:var(--code);padding:2px 6px;border-radius:5px}
 pre code{padding:0;background:none}
 img{max-width:100%;height:auto;display:block;margin:14px 0;border-radius:10px;background:var(--img-bg);padding:var(--img-pad)}
 table{border-collapse:collapse;max-width:100%;margin:14px 0;font-size:13px}
 td,th{border:1px solid var(--line);padding:7px 10px}
 th{background:var(--code);color:var(--title);font-weight:600;text-align:left}
 ul,ol{padding-left:20px;margin:8px 0 14px}
 li{margin-bottom:6px}
 hr{border:0;border-top:1px solid var(--line);margin:18px 0}
 math{font-family:"STIX Two Math","Cambria Math","Times New Roman",Georgia,serif;font-size:1.06em}
 math[display="block"]{display:block;margin:14px 0;text-align:center;overflow-x:auto}
 .video-card{display:inline-flex;align-items:center;gap:8px;padding:10px 14px;margin:12px 0;border-radius:10px;background:var(--code);color:var(--accent);text-decoration:none;font-size:13px;font-weight:500;border:1px solid var(--line)}
 .video-card:hover{background:var(--hover)}
 .toc{margin:0 0 16px;padding:2px 10px;border:1px solid var(--line);border-radius:10px;background:var(--code)}
 .toc-list{display:flex;flex-direction:column;gap:4px;padding:2px 4px 10px}
 .toc-item{color:var(--muted);text-decoration:none;font-size:12.5px;padding:3px 6px;border-radius:6px}
 .toc-item:hover{color:var(--accent);background:var(--hover)}
 .toc-sub{padding-left:18px}
 .extra{margin-top:22px;border-top:1px solid var(--line);padding-top:6px}
 details:not(.toc){border-bottom:1px solid var(--line);padding:4px 0}
 summary{cursor:pointer;padding:10px 4px;color:var(--title);font-weight:500;font-size:13.5px;list-style:none;display:flex;align-items:center;justify-content:space-between;gap:8px;user-select:none;border-radius:8px}
 summary:hover{background:var(--hover)}
 summary::-webkit-details-marker{display:none}
 .sum-left{display:inline-flex;align-items:center;gap:8px}
 .sum-ic{width:15px;height:15px;color:var(--muted);flex:none}
 summary::after{content:"";width:7px;height:7px;border-right:1.7px solid var(--muted);border-bottom:1.7px solid var(--muted);transform:rotate(45deg);margin-right:6px;transition:transform .16s ease;flex:none}
 details[open]>summary::after{transform:rotate(-135deg);margin-top:3px}
 .hint-body{padding:4px 6px 12px 26px;color:var(--text);font-size:13.5px}
 .sim-list{display:flex;flex-direction:column;gap:4px;padding:4px 0 12px}
 .sim-row{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:8px 10px;border-radius:8px;text-decoration:none;color:var(--title)}
 .sim-row:hover{background:var(--hover)}
 .sim-left{display:flex;align-items:center;gap:8px;min-width:0;flex:1}
 .sim-id{color:var(--muted);font-variant-numeric:tabular-nums;font-size:12.5px;flex:none}
 .sim-title{font-weight:500;font-size:13.5px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
 .pill{display:inline-flex;align-items:center;padding:2px 9px;border-radius:10px;font-size:11.5px;font-weight:600;flex:none}
 .pill-easy{color:var(--easy-fg);background:var(--easy-bg)}
 .pill-med{color:var(--med-fg);background:var(--med-bg)}
 .pill-hard{color:var(--hard-fg);background:var(--hard-bg)}
 .pill-pm{color:var(--pm-fg);background:var(--pm-bg);font-size:11px}
 </style></head><body>` + heading + toc.String() + body + `</body></html>`
}
