package render

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/haeward/declic/internal/leetcode"
)

type PlaygroundRef struct {
	Site leetcode.Site
	UUID string
}

var (
	reTOCLine        = regexp.MustCompile(`(?im)^[ \t]*(?:\[TOC\]|\[\[_TOC_\]\])[ \t]*$`)
	reDivIframeBlock = regexp.MustCompile(`(?is)<div\b[^>]*>(?:\s|<div\b[^>]*>)*<iframe\b([^>]*)>(?:\s*</iframe>)?(?:\s|</div>)*`)
	reStandaloneIfr  = regexp.MustCompile(`(?is)<iframe\b([^>]*)>(?:\s*</iframe>)?`)
	reVideoBlock     = regexp.MustCompile(`(?is)(?:<div\b[^>]*>\s*)*<video\b([^>]*)>(.*?)</video>(?:\s*</div>)*`)
	reSourceTag      = regexp.MustCompile(`(?is)<source\b([^>]*)/?>`)
	reAttrSrc        = regexp.MustCompile(`(?i)(?:^|\s)src\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>"']+))`)
	reAttrHref       = regexp.MustCompile(`(?i)(?:^|\s)href\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>"']+))`)
	reAttrAlt        = regexp.MustCompile(`(?i)(?:^|\s)alt\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	reAnchorImg      = regexp.MustCompile(`(?is)<a\b([^>]*)>\s*<img\b([^>]*)/?>\s*</a>`)
	reImgTag         = regexp.MustCompile(`(?is)(?:<div\b[^>]*>\s*)?<img\b([^>]*)/?>(?:\s*</div>)?`)
	reEmptyHTMLBlock = regexp.MustCompile(`(?is)<(?:div|p)\b[^>]*>(?:\s|&nbsp;|&#160;|<br\s*/?>)*</(?:div|p)>`)
	reSlideCarousel  = regexp.MustCompile(`(?s)!\?!([^!\n]+?\.json[^!\n]*?)!\?!`)
	reMultiBlank     = regexp.MustCompile(`\n{3,}`)
	reVideoUUID      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func extractAttr(attrs string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(attrs)
	for i := 1; i < len(m); i++ {
		if m[i] != "" {
			return html.UnescapeString(strings.TrimSpace(m[i]))
		}
	}
	return ""
}

func parsePlaygroundURL(raw string) (PlaygroundRef, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return PlaygroundRef{}, false
	}
	host := strings.ToLower(u.Hostname())
	var site leetcode.Site
	switch host {
	case "leetcode.com", "www.leetcode.com":
		site = leetcode.COM
	case "leetcode.cn", "www.leetcode.cn":
		site = leetcode.CN
	default:
		return PlaygroundRef{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "playground" || !IsSafeSlug(parts[1]) {
		return PlaygroundRef{}, false
	}
	return PlaygroundRef{Site: site, UUID: parts[1]}, true
}

var (
	reLatexNCommand = regexp.MustCompile(`^\\(?:neq|ne\b|not\b|newline\b|nabla|nu\b|neg|notin|nmid|nexists|nolimits|ni\b|natural)`)
	reLatexTCommand = regexp.MustCompile(`^\\(?:times|to\b|text|theta|tau\b|tfrac|tilde|triangle|top\b|therefore|tan\b|tanh\b|tbinom)`)
)

// unescapeLeetCodeMarkdown restores real newlines and tabs when LeetCode's UGC
// GraphQL API returns a Markdown body with literal "\n" / "\t" escape sequences.
func unescapeLeetCodeMarkdown(src string) string {
	realNL := strings.Count(src, "\n")
	litNL := strings.Count(src, `\n`)
	if litNL == 0 || (realNL > 2 && litNL <= realNL*4) {
		return src
	}
	var b strings.Builder
	b.Grow(len(src))
	for i := 0; i < len(src); {
		if src[i] == '\\' && i+1 < len(src) {
			switch src[i+1] {
			case '\\':
				if i+2 < len(src) && (src[i+2] == 'n' || src[i+2] == 't' || src[i+2] == 'r') {
					b.WriteByte('\\')
					b.WriteByte(src[i+2])
					i += 3
					continue
				}
				b.WriteString(`\\`)
				i += 2
				continue
			case 'r':
				if i+3 < len(src) && src[i+2] == '\\' && src[i+3] == 'n' {
					b.WriteByte('\n')
					i += 4
					continue
				}
			case 'n':
				if !reLatexNCommand.MatchString(src[i:]) {
					b.WriteByte('\n')
					i += 2
					continue
				}
			case 't':
				if !reLatexTCommand.MatchString(src[i:]) {
					b.WriteByte('\t')
					i += 2
					continue
				}
			}
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}

// normalizeFenceLine strips LeetCode's trailing "[]" marker from fenced code headers
// (e.g. "```java []" -> "```java", "```python3 []" -> "```python") so syntax highlighting works.
func normalizeFenceLine(line string) string {
	indentLen := len(line) - len(strings.TrimLeft(line, " \t"))
	trim := line[indentLen:]
	if !strings.HasPrefix(trim, "```") {
		return line
	}
	fenceLen := 0
	for fenceLen < len(trim) && trim[fenceLen] == '`' {
		fenceLen++
	}
	info := strings.TrimSpace(trim[fenceLen:])
	if info == "" {
		return line
	}
	if strings.HasPrefix(info, "declic-") || strings.EqualFold(info, "mermaid") {
		return line
	}
	cleaned := strings.TrimSpace(strings.TrimSuffix(info, "[]"))
	if lang := normalizeCodeTabLang(cleaned); lang != "" {
		return line[:indentLen] + strings.Repeat("`", fenceLen) + fenceLanguage(lang)
	}
	if cleaned != info {
		return line[:indentLen] + strings.Repeat("`", fenceLen) + cleaned
	}
	return line
}

// PreprocessArticleMarkdown cleans LeetCode editorial/solution Markdown, extracting
// playground snippets, video walkthroughs, interactive slide decks, and multi-language code tabs.
func PreprocessArticleMarkdown(src string) (string, []PlaygroundRef, []string) {
	normalized := unescapeLeetCodeMarkdown(strings.ReplaceAll(src, "\r\n", "\n"))
	lines := strings.Split(normalized, "\n")

	var out strings.Builder
	var prose strings.Builder
	var refs []PlaygroundRef
	var videos []string
	seenRef := map[string]bool{}
	seenVid := map[string]bool{}
	addVideo := func(u string) {
		if u != "" && !seenVid[u] {
			seenVid[u] = true
			videos = append(videos, u)
		}
	}

	flushProse := func() {
		if prose.Len() == 0 {
			return
		}
		chunk := prose.String()
		prose.Reset()

		// Strip literal [TOC] markers since we render a native collapsible Contents panel.
		chunk = reTOCLine.ReplaceAllString(chunk, "")

		// Convert iframe blocks (video walkthroughs and LeetCode playgrounds).
		replaceIframe := func(full string) string {
			sub := reStandaloneIfr.FindStringSubmatch(full)
			if len(sub) < 2 {
				return ""
			}
			srcURL := extractAttr(sub[1], reAttrSrc)
			if strings.HasPrefix(srcURL, "//") {
				srcURL = "https:" + srcURL
			}
			if ref, ok := parsePlaygroundURL(srcURL); ok {
				key := string(ref.Site) + ":" + ref.UUID
				if !seenRef[key] {
					seenRef[key] = true
					refs = append(refs, ref)
				}
				return fmt.Sprintf("\n\n```declic-playground\n%s %s\n```\n\n", string(ref.Site), ref.UUID)
			}
			if IsTrustedVideoHost(srcURL) {
				addVideo(srcURL)
				return fmt.Sprintf("\n\n```declic-video\n%s\n```\n\n", srcURL)
			}
			return ""
		}
		chunk = reDivIframeBlock.ReplaceAllStringFunc(chunk, replaceIframe)
		chunk = reStandaloneIfr.ReplaceAllStringFunc(chunk, replaceIframe)

		// Convert HTML5 <video> tags (used on leetcode.cn and direct MP4 walkthroughs).
		chunk = reVideoBlock.ReplaceAllStringFunc(chunk, func(full string) string {
			sub := reVideoBlock.FindStringSubmatch(full)
			if len(sub) < 3 {
				return ""
			}
			attrs, inner := sub[1], sub[2]
			srcURL := extractAttr(attrs, reAttrSrc)
			if srcURL == "" {
				if sm := reSourceTag.FindStringSubmatch(inner); len(sm) >= 2 {
					srcURL = extractAttr(sm[1], reAttrSrc)
				}
			}
			if strings.HasPrefix(srcURL, "//") {
				srcURL = "https:" + srcURL
			}
			if !IsTrustedVideoHost(srcURL) && !IsDirectVideoURL(srcURL) {
				return ""
			}
			addVideo(srcURL)
			return fmt.Sprintf("\n\n```declic-video\n%s\n```\n\n", srcURL)
		})

		// Convert LeetCode interactive slide carousels to a native interactive deck marker.
		chunk = reSlideCarousel.ReplaceAllStringFunc(chunk, func(full string) string {
			sub := reSlideCarousel.FindStringSubmatch(full)
			spec := ""
			if len(sub) >= 2 {
				spec = strings.TrimSpace(strings.ReplaceAll(sub[1], "\r", ""))
			}
			if spec == "" || strings.Contains(spec, "`") {
				return ""
			}
			return fmt.Sprintf("\n\n```declic-slides\n%s\n```\n\n", spec)
		})

		// Convert numeric inline math like $0$ or $10^4$ starting with a digit to \(...\) so ExtDollarMath's currency guard does not skip it.
		chunk = convertDigitStartedInlineMath(chunk)

		// Convert HTML <a href="..."><img src="..."></a> (linked video thumbnails or clickable images).
		chunk = reAnchorImg.ReplaceAllStringFunc(chunk, func(full string) string {
			sub := reAnchorImg.FindStringSubmatch(full)
			if len(sub) < 3 {
				return full
			}
			href := normalizeMediaURL(extractAttr(sub[1], reAttrHref))
			imgSrc := normalizeMediaURL(extractAttr(sub[2], reAttrSrc))
			alt := strings.TrimSpace(extractAttr(sub[2], reAttrAlt))
			if block, isVid, ok := buildLinkedMediaBlock(href, imgSrc, alt); ok {
				if isVid {
					addVideo(href)
				}
				return block
			}
			if u, err := url.Parse(imgSrc); err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil {
				cleanAlt := strings.ReplaceAll(alt, "]", "")
				return fmt.Sprintf("\n\n![%s](%s)\n\n", cleanAlt, imgSrc)
			}
			return ""
		})

		// Convert Markdown linked images [![alt](imgURL)](linkURL) and isolate inline images.
		chunk = rewriteMarkdownLinkedAndInlineImages(chunk, addVideo)

		// Convert safe HTTPS HTML <img> tags to standard Markdown images.
		chunk = reImgTag.ReplaceAllStringFunc(chunk, func(full string) string {
			sub := reImgTag.FindStringSubmatch(full)
			if len(sub) < 2 {
				return full
			}
			imgSrc := extractAttr(sub[1], reAttrSrc)
			if strings.HasPrefix(imgSrc, "//") {
				imgSrc = "https:" + imgSrc
			}
			u, err := url.Parse(imgSrc)
			if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
				return ""
			}
			alt := strings.ReplaceAll(extractAttr(sub[1], reAttrAlt), "]", "")
			return fmt.Sprintf("\n\n![%s](%s)\n\n", alt, imgSrc)
		})

		// Drop empty spacer HTML blocks like <div>&nbsp;</div>.
		for {
			next := reEmptyHTMLBlock.ReplaceAllString(chunk, "")
			if next == chunk {
				break
			}
			chunk = next
		}

		out.WriteString(chunk)
	}

	inFence := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			if !inFence {
				flushProse()
				inFence = true
			} else {
				inFence = false
			}
			out.WriteString(line)
			if i+1 < len(lines) {
				out.WriteByte('\n')
			}
			continue
		}
		if inFence {
			out.WriteString(line)
			if i+1 < len(lines) {
				out.WriteByte('\n')
			}
			continue
		}
		prose.WriteString(line)
		if i+1 < len(lines) {
			prose.WriteByte('\n')
		}
	}
	flushProse()

	cleaned := reMultiBlank.ReplaceAllString(out.String(), "\n\n")
	cleaned = stripEmptyVideoSections(cleaned)
	cleaned = mergeConsecutiveCodeBlocks(strings.TrimSpace(cleaned))
	return cleaned, refs, videos
}

func isEmptyVideoHeading(line string) bool {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "#") {
		return false
	}
	s = strings.TrimSpace(strings.TrimLeft(s, "#"))
	low := strings.ToLower(s)
	return low == "video solution" || low == "视频题解"
}

func isHorizontalRuleLine(line string) bool {
	s := strings.TrimSpace(line)
	if s == "" {
		return false
	}
	low := strings.ToLower(s)
	if low == "<hr>" || low == "<hr/>" || low == "<hr />" {
		return true
	}
	if len(s) >= 3 && (strings.Trim(s, "- ") == "" || strings.Trim(s, "* ") == "" || strings.Trim(s, "_ ") == "") {
		return true
	}
	return false
}

// stripEmptyVideoSections drops orphan "Video Solution" headings whose video was
// omitted (e.g., Premium-only editorial videos for non-Premium viewers).
func stripEmptyVideoSections(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	for i := 0; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "```") {
			inFence = !inFence
			out = append(out, lines[i])
			continue
		}
		if !inFence && isEmptyVideoHeading(lines[i]) {
			j := i + 1
			for j < len(lines) && (strings.TrimSpace(lines[j]) == "" || isHorizontalRuleLine(lines[j])) {
				j++
			}
			if j >= len(lines) || strings.HasPrefix(strings.TrimSpace(lines[j]), "#") {
				i = j - 1
				continue
			}
		}
		out = append(out, lines[i])
	}
	return reMultiBlank.ReplaceAllString(strings.Join(out, "\n"), "\n\n")
}

// convertDigitStartedInlineMath converts closed $0$, $1 \le n$, $10^4$ inline formulas
// that start with a digit into \(...\) so ExtDollarMath's currency heuristic (which skips
// $ followed by a digit) still renders them cleanly as inline math while leaving "$5" alone.
func convertDigitStartedInlineMath(prose string) string {
	var out strings.Builder
	i := 0
	for i < len(prose) {
		if prose[i] == '`' {
			start := i
			ticks := 0
			for i < len(prose) && prose[i] == '`' {
				ticks++
				i++
			}
			closeSeq := strings.Repeat("`", ticks)
			if end := strings.Index(prose[i:], closeSeq); end >= 0 {
				i += end + ticks
			} else {
				i = len(prose)
			}
			out.WriteString(prose[start:i])
			continue
		}
		if prose[i] == '$' && i+1 < len(prose) && prose[i+1] == '$' {
			if end := strings.Index(prose[i+2:], "$$"); end >= 0 {
				out.WriteString(prose[i : i+2+end+2])
				i += 2 + end + 2
				continue
			}
		}
		if prose[i] == '$' && (i == 0 || prose[i-1] != '\\') && i+1 < len(prose) && prose[i+1] >= '0' && prose[i+1] <= '9' {
			j := i + 1
			found := -1
			for j < len(prose) {
				if prose[j] == '\n' {
					break
				}
				if prose[j] == '$' && prose[j-1] != '\\' {
					if prose[j-1] != ' ' && prose[j-1] != '\t' && (j+1 >= len(prose) || prose[j+1] < '0' || prose[j+1] > '9') {
						found = j
					}
					break
				}
				j++
			}
			if found > i+1 {
				out.WriteString(`\(`)
				out.WriteString(prose[i+1 : found])
				out.WriteString(`\)`)
				i = found + 1
				continue
			}
		}
		out.WriteByte(prose[i])
		i++
	}
	return out.String()
}

func normalizeMediaURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// Strip optional Markdown title after whitespace inside (url "title").
	if sp := strings.IndexAny(s, " \t"); sp > 0 {
		s = strings.TrimSpace(s[:sp])
	}
	s = strings.Trim(s, "<>")
	if strings.HasPrefix(s, "//") {
		s = "https:" + s
	} else if strings.HasPrefix(s, "http://") {
		if IsTrustedVideoHost("https://" + strings.TrimPrefix(s, "http://")) {
			s = "https://" + strings.TrimPrefix(s, "http://")
		}
	}
	return s
}

func buildLinkedMediaBlock(linkURL, imgURL, alt string) (string, bool, bool) {
	if IsTrustedVideoHost(linkURL) || IsDirectVideoURL(linkURL) {
		return fmt.Sprintf("\n\n```declic-video\n%s\n```\n\n", linkURL), true, true
	}
	if u, err := url.Parse(imgURL); err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil {
		if lu, err := url.Parse(linkURL); err == nil && (lu.Scheme == "https" || lu.Scheme == "http") && lu.Host != "" && lu.User == nil {
			cleanAlt := strings.NewReplacer("\r", "", "\n", " ", "`", "").Replace(alt)
			return fmt.Sprintf("\n\n```declic-linked-image\n%s\n%s\n%s\n```\n\n", linkURL, imgURL, cleanAlt), false, true
		}
	}
	return "", false, false
}

func scanBalanced(s string, start int, open, close byte) (inner string, end int, ok bool) {
	if start >= len(s) || s[start] != open {
		return "", start, false
	}
	depth := 1
	i := start + 1
	for i < len(s) && depth > 0 {
		if s[i] == '\\' && i+1 < len(s) {
			i += 2
			continue
		}
		if s[i] == open {
			depth++
		} else if s[i] == close {
			depth--
			if depth == 0 {
				return s[start+1 : i], i + 1, true
			}
		}
		i++
	}
	return "", start, false
}

func rewriteMarkdownLinkedAndInlineImages(s string, addVideo func(string)) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		// Linked image: [![alt](imgURL)](linkURL)
		if i+2 < len(s) && s[i] == '[' && s[i+1] == '!' && s[i+2] == '[' {
			alt, p1, ok1 := scanBalanced(s, i+2, '[', ']')
			if ok1 && p1 < len(s) && s[p1] == '(' {
				imgRaw, p2, ok2 := scanBalanced(s, p1, '(', ')')
				if ok2 && p2 < len(s) && s[p2] == ']' {
					p3 := p2 + 1
					if p3 < len(s) && s[p3] == '(' {
						linkRaw, p4, ok3 := scanBalanced(s, p3, '(', ')')
						if ok3 {
							imgURL := normalizeMediaURL(imgRaw)
							linkURL := normalizeMediaURL(linkRaw)
							if block, isVid, ok := buildLinkedMediaBlock(linkURL, imgURL, strings.TrimSpace(alt)); ok {
								if isVid && addVideo != nil {
									addVideo(linkURL)
								}
								out.WriteString(block)
								i = p4
								continue
							}
						}
					}
				}
			}
		}
		// Standalone image inline with text on same line -> isolate onto its own paragraph so markdown.View renders it as an image instead of [alt].
		if i+1 < len(s) && s[i] == '!' && s[i+1] == '[' {
			alt, p1, ok1 := scanBalanced(s, i+1, '[', ']')
			if ok1 && p1 < len(s) && s[p1] == '(' {
				imgRaw, p2, ok2 := scanBalanced(s, p1, '(', ')')
				if ok2 {
					rawTarget := strings.TrimSpace(imgRaw)
					if reVideoUUID.MatchString(rawTarget) {
						out.WriteString("\n\n```declic-cn-video\n")
						out.WriteString(rawTarget)
						out.WriteString("\n```\n\n")
						i = p2
						continue
					}
					imgURL := normalizeMediaURL(imgRaw)
					if IsDirectVideoURL(imgURL) {
						if addVideo != nil {
							addVideo(imgURL)
						}
						out.WriteString("\n\n```declic-video\n")
						out.WriteString(imgURL)
						out.WriteString("\n```\n\n")
						i = p2
						continue
					}
					if u, err := url.Parse(imgURL); err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil {
						// Check if preceded/followed by non-whitespace on the same line.
						prevNonWS := i > 0 && s[i-1] != '\n'
						nextNonWS := p2 < len(s) && s[p2] != '\n'
						if prevNonWS || nextNonWS {
							out.WriteString("\n\n![")
							out.WriteString(alt)
							out.WriteString("](")
							out.WriteString(imgURL)
							out.WriteString(")\n\n")
							i = p2
							continue
						}
					}
				}
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func normalizeCodeTabLang(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.IndexAny(s, " \t[]"); idx > 0 {
		s = s[:idx]
	}
	switch s {
	case "c++", "cpp", "cxx":
		return "cpp"
	case "c#", "cs", "csharp":
		return "csharp"
	case "py", "py3", "python3":
		return "python3"
	case "python":
		return "python"
	case "go", "golang":
		return "golang"
	case "js", "javascript":
		return "javascript"
	case "ts", "typescript":
		return "typescript"
	case "rs":
		return "rust"
	case "kt":
		return "kotlin"
	case "rb":
		return "ruby"
	case "sh":
		return "bash"
	case "sql":
		return "mysql"
	case "java", "c", "rust", "kotlin", "swift", "ruby", "php", "dart", "scala", "elixir", "erlang", "racket", "mysql", "postgresql", "bash":
		return s
	default:
		return ""
	}
}

func isLanguageLabelLine(line string) bool {
	s := strings.TrimSpace(line)
	if s == "" {
		return true
	}
	s = strings.TrimLeft(s, "#")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "*_`[](): ")
	if s == "" || len(s) > 24 {
		return false
	}
	return normalizeCodeTabLang(s) != ""
}

// mergeConsecutiveCodeBlocks groups 2+ adjacent fenced code blocks in different programming
// languages (commonly used in LeetCode Solutions) into a single tabbed declic-codetabs block.
func mergeConsecutiveCodeBlocks(src string) string {
	type segment struct {
		isCode bool
		lang   string
		code   string
		raw    string
	}
	lines := strings.Split(src, "\n")
	var segs []segment
	var textBuf []string
	flushText := func() {
		if len(textBuf) > 0 {
			segs = append(segs, segment{raw: strings.Join(textBuf, "\n")})
			textBuf = nil
		}
	}
	for i := 0; i < len(lines); {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "```") {
			fenceLen := 0
			for fenceLen < len(trim) && trim[fenceLen] == '`' {
				fenceLen++
			}
			info := strings.TrimSpace(trim[fenceLen:])
			closeMark := strings.Repeat("`", fenceLen)
			j := i + 1
			for j < len(lines) {
				t2 := strings.TrimSpace(lines[j])
				if strings.HasPrefix(t2, closeMark) && strings.Trim(t2, "`") == "" {
					break
				}
				j++
			}
			if j < len(lines) {
				flushText()
				lang := normalizeCodeTabLang(info)
				code := strings.Join(lines[i+1:j], "\n")
				normLines := append([]string{normalizeFenceLine(lines[i])}, lines[i+1:j+1]...)
				raw := strings.Join(normLines, "\n")
				segs = append(segs, segment{isCode: lang != "", lang: lang, code: code, raw: raw})
				i = j + 1
				continue
			}
		}
		textBuf = append(textBuf, lines[i])
		i++
	}
	flushText()

	isOnlyLangLabelOrBlank := func(raw string) bool {
		for _, l := range strings.Split(raw, "\n") {
			if !isLanguageLabelLine(l) {
				return false
			}
		}
		return true
	}

	var out []string
	for i := 0; i < len(segs); {
		if !segs[i].isCode {
			out = append(out, segs[i].raw)
			i++
			continue
		}
		// Collect a run of consecutive distinct-language code blocks.
		run := []leetcode.Snippet{{Language: segs[i].lang, Code: segs[i].code}}
		seen := map[string]bool{segs[i].lang: true}
		endIdx := i
		k := i + 1
		for k < len(segs) {
			if segs[k].isCode {
				if seen[segs[k].lang] {
					break
				}
				seen[segs[k].lang] = true
				run = append(run, leetcode.Snippet{Language: segs[k].lang, Code: segs[k].code})
				endIdx = k
				k++
				continue
			}
			// Allow blank lines or a short language-only label between blocks if followed by another distinct code block.
			if k+1 < len(segs) && segs[k+1].isCode && !seen[segs[k+1].lang] && isOnlyLangLabelOrBlank(segs[k].raw) {
				k++
				continue
			}
			break
		}
		if len(run) >= 2 {
			// Also trim a trailing standalone language-only label right before the first block if present.
			if len(out) > 0 {
				prevLines := strings.Split(out[len(out)-1], "\n")
				for len(prevLines) > 0 && strings.TrimSpace(prevLines[len(prevLines)-1]) == "" {
					prevLines = prevLines[:len(prevLines)-1]
				}
				if len(prevLines) > 0 && isLanguageLabelLine(prevLines[len(prevLines)-1]) && strings.TrimSpace(prevLines[len(prevLines)-1]) != "" {
					prevLines = prevLines[:len(prevLines)-1]
					out[len(out)-1] = strings.Join(prevLines, "\n")
				}
			}
			if rawJSON, err := json.Marshal(run); err == nil {
				out = append(out, "```declic-codetabs\n"+string(rawJSON)+"\n```")
				i = endIdx + 1
				continue
			}
		}
		out = append(out, segs[i].raw)
		i++
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func fenceLanguage(slug string) string {
	switch slug {
	case "python3", "python", "pythondata":
		return "python"
	case "golang":
		return "go"
	case "mysql", "mssql", "oraclesql", "postgresql":
		return "sql"
	default:
		return slug
	}
}

// FencedCodeMarkdown wraps source code in a safely escaped Markdown code fence.
func FencedCodeMarkdown(langSlug, code string) string {
	fence := "```"
	for strings.Contains(code, fence) {
		fence += "`"
	}
	body := strings.TrimRight(code, "\n")
	return fence + fenceLanguage(langSlug) + "\n" + body + "\n" + fence
}
