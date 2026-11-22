package render

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	xhtml "golang.org/x/net/html"
)

type mathKind int

const (
	mathGroup mathKind = iota
	mathIdent
	mathNumber
	mathOp
	mathText
	mathFrac
	mathSqrt
	mathScript // base + sub + sup
	mathCases
)

type mathNode struct {
	kind     mathKind
	text     string
	children []*mathNode
	base     *mathNode
	sub      *mathNode
	sup      *mathNode
	num      *mathNode
	den      *mathNode
	index    *mathNode
	rows     [][]*mathNode
}

var texSymbols = map[string]string{
	// Greek lowercase
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε", "varepsilon": "ε",
	"zeta": "ζ", "eta": "η", "theta": "θ", "vartheta": "ϑ", "iota": "ι", "kappa": "κ",
	"lambda": "λ", "mu": "μ", "nu": "ν", "xi": "ξ", "pi": "π", "varpi": "ϖ",
	"rho": "ρ", "varrho": "ϱ", "sigma": "σ", "varsigma": "ς", "tau": "τ", "upsilon": "υ",
	"phi": "φ", "varphi": "φ", "chi": "χ", "psi": "ψ", "omega": "ω",
	// Greek uppercase
	"Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ", "Pi": "Π",
	"Sigma": "Σ", "Upsilon": "Υ", "Phi": "Φ", "Psi": "Ψ", "Omega": "Ω",
	// Relations & operators
	"le": "≤", "leq": "≤", "leqslant": "≤", "ge": "≥", "geq": "≥", "geqslant": "≥",
	"ne": "≠", "neq": "≠", "approx": "≈", "equiv": "≡", "sim": "∼", "simeq": "≃", "cong": "≅",
	"ll": "≪", "gg": "≫", "propto": "∝", "perp": "⊥", "parallel": "∥",
	"times": "×", "cdot": "·", "cdots": "⋯", "dots": "…", "ldots": "…", "vdots": "⋮", "ddots": "⋱",
	"div": "÷", "pm": "±", "mp": "∓", "oplus": "⊕", "otimes": "⊗", "ominus": "⊖", "circ": "∘", "bullet": "•",
	"star": "⋆", "ast": "∗", "wedge": "∧", "land": "∧", "vee": "∨", "lor": "∨", "neg": "¬", "lnot": "¬",
	"mid": "∣", "nmid": "∤",
	// Set theory & logic
	"in": "∈", "notin": "∉", "ni": "∋", "subset": "⊂", "subseteq": "⊆", "supset": "⊃", "supseteq": "⊇",
	"cup": "∪", "cap": "∩", "setminus": "∖", "emptyset": "∅", "varnothing": "∅",
	"forall": "∀", "exists": "∃", "nexists": "∄", "therefore": "∴", "because": "∵",
	// Arrows
	"to": "→", "rightarrow": "→", "leftarrow": "←", "leftrightarrow": "↔",
	"Rightarrow": "⇒", "Leftarrow": "⇐", "Leftrightarrow": "⇔", "iff": "⇔", "implies": "⇒",
	"mapsto": "↦", "uparrow": "↑", "downarrow": "↓",
	// Big operators & misc
	"sum": "∑", "prod": "∏", "int": "∫", "iint": "∬", "oint": "∮", "bigcup": "⋃", "bigcap": "⋂",
	"infty": "∞", "partial": "∂", "nabla": "∇", "prime": "′", "angle": "∠", "degree": "°",
	"lfloor": "⌊", "rfloor": "⌋", "lceil": "⌈", "rceil": "⌉", "langle": "⟨", "rangle": "⟩",
	"lbrace": "{", "rbrace": "}", "{": "{", "}": "}", "|": "‖", "%": "%", "&": "&", "#": "#", "_": "_",
	"quad": "  ", "qquad": "    ", ",": " ", ";": " ", ":": " ", "!": "", " ": " ",
}

var texFunctions = map[string]string{
	"log": "log", "ln": "ln", "lg": "lg", "exp": "exp",
	"sin": "sin", "cos": "cos", "tan": "tan", "cot": "cot", "sec": "sec", "csc": "csc",
	"arcsin": "arcsin", "arccos": "arccos", "arctan": "arctan",
	"min": "min", "max": "max", "inf": "inf", "sup": "sup", "lim": "lim", "limsup": "lim sup", "liminf": "lim inf",
	"gcd": "gcd", "lcm": "lcm", "det": "det", "dim": "dim", "ker": "ker", "deg": "deg", "arg": "arg",
	"mod": "mod", "bmod": "mod", "Pr": "Pr",
}

var blackboardMap = map[rune]string{
	'N': "ℕ", 'Z': "ℤ", 'Q': "ℚ", 'R': "ℝ", 'C': "ℂ", 'P': "ℙ", 'E': "𝔼", 'F': "𝔽",
}

var superscriptRunes = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴', '5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'+': '⁺', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾',
	'a': 'ᵃ', 'b': 'ᵇ', 'c': 'ᶜ', 'd': 'ᵈ', 'e': 'ᵉ', 'f': 'ᶠ', 'g': 'ᵍ', 'h': 'ʰ', 'i': 'ⁱ', 'j': 'ʲ',
	'k': 'ᵏ', 'l': 'ˡ', 'm': 'ᵐ', 'n': 'ⁿ', 'o': 'ᵒ', 'p': 'ᵖ', 'r': 'ʳ', 's': 'ˢ', 't': 'ᵗ', 'u': 'ᵘ',
	'v': 'ᵛ', 'w': 'ʷ', 'x': 'ˣ', 'y': 'ʸ', 'z': 'ᶻ',
	'A': 'ᴬ', 'B': 'ᴮ', 'D': 'ᴰ', 'E': 'ᴱ', 'G': 'ᴳ', 'H': 'ᴴ', 'I': 'ᴵ', 'J': 'ᴶ', 'K': 'ᴷ', 'L': 'ᴸ',
	'M': 'ᴹ', 'N': 'ᴺ', 'O': 'ᴼ', 'P': 'ᴾ', 'R': 'ᴿ', 'T': 'ᵀ', 'U': 'ᵁ', 'V': 'ⱽ', 'W': 'ᵂ',
}

var subscriptRunes = map[rune]rune{
	'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄', '5': '₅', '6': '₆', '7': '₇', '8': '₈', '9': '₉',
	'+': '₊', '-': '₋', '=': '₌', '(': '₍', ')': '₎',
	'a': 'ₐ', 'e': 'ₑ', 'h': 'ₕ', 'i': 'ᵢ', 'j': 'ⱼ', 'k': 'ₖ', 'l': 'ₗ', 'm': 'ₘ', 'n': 'ₙ',
	'o': 'ₒ', 'p': 'ₚ', 'r': 'ᵣ', 's': 'ₛ', 't': 'ₜ', 'u': 'ᵤ', 'v': 'ᵥ', 'x': 'ₓ',
}

type texParser struct {
	src string
	pos int
}

func parseTeX(src string) *mathNode {
	p := &texParser{src: strings.TrimSpace(src)}
	return p.parseSequence("")
}

func (p *texParser) skipSpaces() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t' || p.src[p.pos] == '\n' || p.src[p.pos] == '\r') {
		p.pos++
	}
}

func (p *texParser) parseSequence(stop string) *mathNode {
	group := &mathNode{kind: mathGroup}
	for p.pos < len(p.src) {
		p.skipSpaces()
		if p.pos >= len(p.src) {
			break
		}
		if stop != "" && strings.HasPrefix(p.src[p.pos:], stop) {
			break
		}
		if p.src[p.pos] == '}' {
			break
		}
		if p.src[p.pos] == '^' || p.src[p.pos] == '_' {
			op := p.src[p.pos]
			p.pos++
			arg := p.parseAtom()
			if len(group.children) == 0 {
				group.children = append(group.children, &mathNode{kind: mathIdent, text: ""})
			}
			last := group.children[len(group.children)-1]
			var scr *mathNode
			if last.kind == mathScript {
				scr = last
			} else {
				scr = &mathNode{kind: mathScript, base: last}
				group.children[len(group.children)-1] = scr
			}
			if op == '^' {
				scr.sup = arg
			} else {
				scr.sub = arg
			}
			continue
		}
		atom := p.parseAtom()
		if atom != nil {
			group.children = append(group.children, atom)
		}
	}
	return group
}

func (p *texParser) parseGroupArg() *mathNode {
	p.skipSpaces()
	if p.pos >= len(p.src) {
		return &mathNode{kind: mathGroup}
	}
	if p.src[p.pos] == '{' {
		p.pos++
		node := p.parseSequence("}")
		if p.pos < len(p.src) && p.src[p.pos] == '}' {
			p.pos++
		}
		return node
	}
	return p.parseAtom()
}

func (p *texParser) parseBracketArg() *mathNode {
	p.skipSpaces()
	if p.pos < len(p.src) && p.src[p.pos] == '[' {
		p.pos++
		node := p.parseSequence("]")
		if p.pos < len(p.src) && p.src[p.pos] == ']' {
			p.pos++
		}
		return node
	}
	return nil
}

func (p *texParser) parseRawBraceText() string {
	p.skipSpaces()
	if p.pos >= len(p.src) || p.src[p.pos] != '{' {
		return ""
	}
	p.pos++
	start := p.pos
	depth := 1
	for p.pos < len(p.src) && depth > 0 {
		if p.src[p.pos] == '{' {
			depth++
		} else if p.src[p.pos] == '}' {
			depth--
			if depth == 0 {
				out := p.src[start:p.pos]
				p.pos++
				return out
			}
		}
		p.pos++
	}
	return p.src[start:]
}

func (p *texParser) parseAtom() *mathNode {
	p.skipSpaces()
	if p.pos >= len(p.src) {
		return nil
	}
	ch := p.src[p.pos]
	if ch == '{' {
		p.pos++
		node := p.parseSequence("}")
		if p.pos < len(p.src) && p.src[p.pos] == '}' {
			p.pos++
		}
		return node
	}
	if ch == '\\' {
		return p.parseCommand()
	}
	if ch >= '0' && ch <= '9' {
		start := p.pos
		for p.pos < len(p.src) && ((p.src[p.pos] >= '0' && p.src[p.pos] <= '9') || p.src[p.pos] == '.') {
			p.pos++
		}
		return &mathNode{kind: mathNumber, text: p.src[start:p.pos]}
	}
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
		p.pos++
		return &mathNode{kind: mathIdent, text: string(ch)}
	}
	r, size := utf8.DecodeRuneInString(p.src[p.pos:])
	p.pos += size
	switch r {
	case '-':
		return &mathNode{kind: mathOp, text: "−"}
	case '\'':
		return &mathNode{kind: mathOp, text: "′"}
	default:
		return &mathNode{kind: mathOp, text: string(r)}
	}
}

func (p *texParser) parseCommand() *mathNode {
	p.pos++ // skip '\'
	if p.pos >= len(p.src) {
		return nil
	}
	if p.src[p.pos] == '\\' {
		p.pos++
		return &mathNode{kind: mathOp, text: "\n"}
	}
	start := p.pos
	r, size := utf8.DecodeRuneInString(p.src[p.pos:])
	if !unicode.IsLetter(r) {
		p.pos += size
		cmd := p.src[start:p.pos]
		if sym, ok := texSymbols[cmd]; ok {
			return &mathNode{kind: mathOp, text: sym}
		}
		return &mathNode{kind: mathOp, text: cmd}
	}
	for p.pos < len(p.src) {
		r, size = utf8.DecodeRuneInString(p.src[p.pos:])
		if !unicode.IsLetter(r) {
			break
		}
		p.pos += size
	}
	cmd := p.src[start:p.pos]
	switch cmd {
	case "frac", "dfrac", "tfrac":
		num := p.parseGroupArg()
		den := p.parseGroupArg()
		return &mathNode{kind: mathFrac, num: num, den: den}
	case "binom", "dbinom", "tbinom":
		n := p.parseGroupArg()
		k := p.parseGroupArg()
		return &mathNode{kind: mathGroup, children: []*mathNode{
			{kind: mathOp, text: "("},
			{kind: mathFrac, text: "binom", num: n, den: k},
			{kind: mathOp, text: ")"},
		}}
	case "sqrt":
		idx := p.parseBracketArg()
		body := p.parseGroupArg()
		return &mathNode{kind: mathSqrt, index: idx, base: body}
	case "textit", "emph":
		raw := p.parseRawBraceText()
		return &mathNode{kind: mathIdent, text: raw}
	case "text", "textrm", "textbf", "mathrm", "mathbf", "operatorname", "texttt", "mathsf":
		raw := p.parseRawBraceText()
		return &mathNode{kind: mathText, text: raw}
	case "mathcal", "mathscr", "mathit":
		return p.parseGroupArg()
	case "mathbb":
		raw := p.parseRawBraceText()
		var b strings.Builder
		for _, r := range raw {
			if sym, ok := blackboardMap[r]; ok {
				b.WriteString(sym)
			} else {
				b.WriteRune(r)
			}
		}
		return &mathNode{kind: mathIdent, text: b.String()}
	case "pmod":
		arg := p.parseGroupArg()
		return &mathNode{kind: mathGroup, children: []*mathNode{
			{kind: mathText, text: " (mod "},
			arg,
			{kind: mathOp, text: ")"},
		}}
	case "left", "right":
		p.skipSpaces()
		if p.pos < len(p.src) && p.src[p.pos] == '.' {
			p.pos++
		}
		return nil
	case "big", "Big", "bigg", "Bigg", "displaystyle", "textstyle", "limits", "nolimits":
		return nil
	case "overline", "bar", "hat", "tilde", "vec":
		body := p.parseGroupArg()
		return body
	case "begin":
		env := p.parseRawBraceText()
		if env == "array" {
			// Consume mandatory column alignment specification like {l} or {cc}.
			_ = p.parseRawBraceText()
		}
		endTag := `\end{` + env + `}`
		endIdx := strings.Index(p.src[p.pos:], endTag)
		var inner string
		if endIdx >= 0 {
			inner = p.src[p.pos : p.pos+endIdx]
			p.pos += endIdx + len(endTag)
		} else {
			inner = p.src[p.pos:]
			p.pos = len(p.src)
		}
		if env == "cases" || env == "array" || strings.Contains(env, "matrix") || env == "aligned" || env == "align" || env == "align*" {
			rawRows := strings.Split(inner, `\\`)
			var rows [][]*mathNode
			for _, rr := range rawRows {
				rr = strings.TrimSpace(rr)
				if rr == "" {
					continue
				}
				cols := strings.Split(rr, "&")
				var parsed []*mathNode
				for _, col := range cols {
					parsed = append(parsed, parseTeX(col))
				}
				rows = append(rows, parsed)
			}
			return &mathNode{kind: mathCases, text: env, rows: rows}
		}
		return parseTeX(inner)
	}
	if fn, ok := texFunctions[cmd]; ok {
		return &mathNode{kind: mathText, text: fn}
	}
	if sym, ok := texSymbols[cmd]; ok {
		return &mathNode{kind: mathOp, text: sym}
	}
	return &mathNode{kind: mathText, text: cmd}
}

// texToMathMLNode builds a safe Presentation MathML DOM node directly from TeX.
func texToMathMLNode(tex string, display bool) *xhtml.Node {
	el := &xhtml.Node{Type: xhtml.ElementNode, Data: "math"}
	if display {
		el.Attr = append(el.Attr, xhtml.Attribute{Key: "display", Val: "block"})
	}
	appendMathMLNode(el, parseTeX(tex))
	return el
}

func appendMathMLNode(parent *xhtml.Node, n *mathNode) {
	if n == nil {
		return
	}
	mk := func(tag string) *xhtml.Node {
		node := &xhtml.Node{Type: xhtml.ElementNode, Data: tag}
		parent.AppendChild(node)
		return node
	}
	mkText := func(tag, text string) {
		node := mk(tag)
		node.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: text})
	}
	switch n.kind {
	case mathGroup:
		if len(n.children) == 1 {
			appendMathMLNode(parent, n.children[0])
			return
		}
		row := mk("mrow")
		for _, ch := range n.children {
			appendMathMLNode(row, ch)
		}
	case mathIdent:
		node := mk("mi")
		if len([]rune(n.text)) > 1 {
			node.Attr = append(node.Attr, xhtml.Attribute{Key: "mathvariant", Val: "italic"})
		}
		node.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: n.text})
	case mathNumber:
		mkText("mn", n.text)
	case mathOp:
		mkText("mo", n.text)
	case mathText:
		mkText("mtext", n.text)
	case mathFrac:
		f := mk("mfrac")
		if n.text == "binom" {
			f.Attr = append(f.Attr, xhtml.Attribute{Key: "linethickness", Val: "0"})
		}
		nr := &xhtml.Node{Type: xhtml.ElementNode, Data: "mrow"}
		dr := &xhtml.Node{Type: xhtml.ElementNode, Data: "mrow"}
		f.AppendChild(nr)
		f.AppendChild(dr)
		appendMathMLNode(nr, n.num)
		appendMathMLNode(dr, n.den)
	case mathSqrt:
		if n.index != nil {
			r := mk("mroot")
			br := &xhtml.Node{Type: xhtml.ElementNode, Data: "mrow"}
			ir := &xhtml.Node{Type: xhtml.ElementNode, Data: "mrow"}
			r.AppendChild(br)
			r.AppendChild(ir)
			appendMathMLNode(br, n.base)
			appendMathMLNode(ir, n.index)
		} else {
			r := mk("msqrt")
			appendMathMLNode(r, n.base)
		}
	case mathScript:
		switch {
		case n.sub != nil && n.sup != nil:
			s := mk("msubsup")
			appendMathMLNode(s, n.base)
			appendMathMLNode(s, n.sub)
			appendMathMLNode(s, n.sup)
		case n.sub != nil:
			s := mk("msub")
			appendMathMLNode(s, n.base)
			appendMathMLNode(s, n.sub)
		case n.sup != nil:
			s := mk("msup")
			appendMathMLNode(s, n.base)
			appendMathMLNode(s, n.sup)
		default:
			appendMathMLNode(parent, n.base)
		}
	case mathCases:
		row := mk("mrow")
		if n.text == "cases" {
			mo := &xhtml.Node{Type: xhtml.ElementNode, Data: "mo"}
			mo.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: "{"})
			row.AppendChild(mo)
		}
		tbl := &xhtml.Node{Type: xhtml.ElementNode, Data: "mtable"}
		row.AppendChild(tbl)
		for _, r := range n.rows {
			mtr := &xhtml.Node{Type: xhtml.ElementNode, Data: "mtr"}
			tbl.AppendChild(mtr)
			for _, cell := range r {
				mtd := &xhtml.Node{Type: xhtml.ElementNode, Data: "mtd"}
				mtr.AppendChild(mtd)
				appendMathMLNode(mtd, cell)
			}
		}
	}
}

func mapRunes(s string, table map[rune]rune) (string, bool) {
	var b strings.Builder
	for _, r := range s {
		if r == ' ' {
			continue
		}
		if r == '−' {
			r = '-'
		}
		mapped, ok := table[r]
		if !ok {
			return "", false
		}
		b.WriteRune(mapped)
	}
	return b.String(), b.Len() > 0
}

func mathNodeUnicode(n *mathNode) string {
	if n == nil {
		return ""
	}
	switch n.kind {
	case mathGroup:
		var b strings.Builder
		for i, ch := range n.children {
			part := mathNodeUnicode(ch)
			if part == "" {
				continue
			}
			if i > 0 && needsMathSpace(n.children[i-1], ch) {
				b.WriteByte(' ')
			}
			b.WriteString(part)
		}
		return b.String()
	case mathIdent, mathNumber, mathText, mathOp:
		return n.text
	case mathFrac:
		num := mathNodeUnicode(n.num)
		den := mathNodeUnicode(n.den)
		if n.text == "binom" {
			return "C(" + num + ", " + den + ")"
		}
		if len([]rune(num)) > 2 && !strings.HasPrefix(num, "(") {
			num = "(" + num + ")"
		}
		if len([]rune(den)) > 2 && !strings.HasPrefix(den, "(") {
			den = "(" + den + ")"
		}
		return num + " / " + den
	case mathSqrt:
		body := mathNodeUnicode(n.base)
		prefix := "√"
		if n.index != nil {
			if sup, ok := mapRunes(mathNodeUnicode(n.index), superscriptRunes); ok {
				prefix = sup + "√"
			}
		}
		if len([]rune(body)) <= 2 {
			return prefix + body
		}
		return prefix + "(" + body + ")"
	case mathScript:
		var b strings.Builder
		b.WriteString(mathNodeUnicode(n.base))
		if n.sub != nil {
			sub := mathNodeUnicode(n.sub)
			if mapped, ok := mapRunes(sub, subscriptRunes); ok {
				b.WriteString(mapped)
			} else {
				b.WriteByte('_')
				b.WriteString(wrapIfComplex(sub))
			}
		}
		if n.sup != nil {
			sup := mathNodeUnicode(n.sup)
			if mapped, ok := mapRunes(sup, superscriptRunes); ok {
				b.WriteString(mapped)
			} else {
				b.WriteByte('^')
				b.WriteString(wrapIfComplex(sup))
			}
		}
		return b.String()
	case mathCases:
		var lines []string
		for _, row := range n.rows {
			var cols []string
			for _, cell := range row {
				cols = append(cols, strings.TrimSpace(mathNodeUnicode(cell)))
			}
			lines = append(lines, strings.Join(cols, "   "))
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func wrapIfComplex(s string) string {
	if len([]rune(s)) <= 1 {
		return s
	}
	return "(" + s + ")"
}

func needsMathSpace(prev, cur *mathNode) bool {
	if prev == nil || cur == nil {
		return false
	}
	spacedOps := "=<>≤≥≠≈≡+−×÷±∓→←↔⇒⇔∈∉⊂⊆∪∩∧∨"
	if prev.kind == mathOp && len(prev.text) > 0 && strings.Contains(spacedOps, prev.text) {
		return true
	}
	if cur.kind == mathOp && len(cur.text) > 0 && strings.Contains(spacedOps, cur.text) {
		return true
	}
	if prev.kind == mathOp && prev.text == "," {
		return true
	}
	if prev.kind == mathText && (cur.kind == mathIdent || cur.kind == mathNumber || cur.kind == mathGroup) {
		return !strings.HasSuffix(prev.text, " ") && !strings.HasSuffix(prev.text, "(")
	}
	return false
}

func isPlainPrice(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' && r != ',' {
			return false
		}
	}
	return true
}

const mathSerifFont = "Times New Roman"

type inlineMathKey struct {
	tex  string
	size int // size * 10
}

type cachedInlineMath struct {
	layout  *ui.TextLayout
	width   float32
	ascent  float32
	descent float32
}

var inlineMathCache sync.Map

func appendInlineMathSpans(spans *[]ui.Span, n *mathNode, size float32) {
	if n == nil {
		return
	}
	addSpan := func(text string, italic bool, sz float32) {
		if text == "" {
			return
		}
		if len(*spans) > 0 {
			last := &(*spans)[len(*spans)-1]
			if last.Italic == italic && last.Size == sz {
				last.Text += text
				return
			}
		}
		*spans = append(*spans, ui.Span{Text: text, Font: mathSerifFont, Size: sz, Italic: italic})
	}
	switch n.kind {
	case mathGroup:
		for i, ch := range n.children {
			if i > 0 && needsMathSpace(n.children[i-1], ch) {
				addSpan(" ", false, size)
			}
			appendInlineMathSpans(spans, ch, size)
		}
	case mathIdent:
		addSpan(n.text, true, size)
	case mathNumber, mathText, mathOp:
		addSpan(n.text, false, size)
	case mathFrac:
		if n.text == "binom" {
			addSpan("C(", false, size)
			appendInlineMathSpans(spans, n.num, size)
			addSpan(", ", false, size)
			appendInlineMathSpans(spans, n.den, size)
			addSpan(")", false, size)
			return
		}
		numText := mathNodeUnicode(n.num)
		denText := mathNodeUnicode(n.den)
		wrapNum := len([]rune(numText)) > 2 && !strings.HasPrefix(numText, "(")
		wrapDen := len([]rune(denText)) > 2 && !strings.HasPrefix(denText, "(")
		if wrapNum {
			addSpan("(", false, size)
		}
		appendInlineMathSpans(spans, n.num, size)
		if wrapNum {
			addSpan(")", false, size)
		}
		addSpan(" / ", false, size)
		if wrapDen {
			addSpan("(", false, size)
		}
		appendInlineMathSpans(spans, n.den, size)
		if wrapDen {
			addSpan(")", false, size)
		}
	case mathSqrt:
		if n.index != nil {
			if sup, ok := mapRunes(mathNodeUnicode(n.index), superscriptRunes); ok {
				addSpan(sup, false, size)
			}
		}
		addSpan("√", false, size)
		body := mathNodeUnicode(n.base)
		if len([]rune(body)) > 2 {
			addSpan("(", false, size)
			appendInlineMathSpans(spans, n.base, size)
			addSpan(")", false, size)
		} else {
			appendInlineMathSpans(spans, n.base, size)
		}
	case mathScript:
		appendInlineMathSpans(spans, n.base, size)
		if n.sub != nil {
			sub := mathNodeUnicode(n.sub)
			if mapped, ok := mapRunes(sub, subscriptRunes); ok {
				addSpan(mapped, false, size)
			} else {
				addSpan("_", false, size)
				appendInlineMathSpans(spans, n.sub, size*0.82)
			}
		}
		if n.sup != nil {
			sup := mathNodeUnicode(n.sup)
			if mapped, ok := mapRunes(sup, superscriptRunes); ok {
				addSpan(mapped, false, size)
			} else {
				addSpan("^", false, size)
				appendInlineMathSpans(spans, n.sup, size*0.82)
			}
		}
	case mathCases:
		addSpan(mathNodeUnicode(n), false, size)
	}
}

// InlineMathObject renders inline TeX ($...$ and \(...\)) inside Markdown paragraphs
// using unified Times New Roman serif math typography (italic variables, upright numbers,
// brackets, and operators) aligned on the text baseline with no background box.
func InlineMathObject(tex string, size float32) (markdown.InlineObject, bool) {
	trimmed := strings.TrimSpace(tex)
	if trimmed == "" {
		return markdown.InlineObject{}, false
	}
	mathSize := size * 1.05
	key := inlineMathKey{tex: trimmed, size: int(mathSize*10 + 0.5)}
	var cached *cachedInlineMath
	if v, ok := inlineMathCache.Load(key); ok {
		cached = v.(*cachedInlineMath)
	} else {
		var spans []ui.Span
		appendInlineMathSpans(&spans, parseTeX(trimmed), mathSize)
		if len(spans) == 0 {
			return markdown.InlineObject{}, false
		}
		layout := ui.ShapeRichText(spans, ui.Font{Family: mathSerifFont, Size: mathSize}, 0)
		w, h := layout.Size()
		if w <= 0 || h <= 0 {
			return markdown.InlineObject{}, false
		}
		ascent, descent := h*0.8, h*0.2
		if lines := layout.Lines(); len(lines) > 0 {
			ascent = lines[0].Ascent
			descent = lines[0].Descent
		}
		cached = &cachedInlineMath{layout: layout, width: w, ascent: ascent, descent: descent}
		inlineMathCache.Store(key, cached)
	}
	cLayout := cached.layout
	return markdown.InlineObject{
		Width:  cached.width,
		Height: cached.ascent,
		Depth:  cached.descent,
		Draw: func(p *ui.Painter, r ui.Rect) {
			p.TextLayout(cLayout, r.X, r.Y, p.Theme().Text.Alpha(.86))
		},
	}, true
}

// RenderDisplayMath renders a display TeX block natively in MyGo UI with classic serif
// math typography directly on the page canvas (no background card or border, matching LeetCode).
func RenderDisplayMath(c *ui.Context, tex string) {
	t := c.Theme()
	root := parseTeX(tex)
	ui.Column(c).FillWidth().Padding(8, 4).Center().Children(func() {
		ui.ScrollHorizontal(c).Scrollbars(false).Children(func() {
			renderMathNodeUI(c, root, 16, t.Text)
		})
	})
}

func renderMathNodeUI(c *ui.Context, n *mathNode, size float32, fg ui.Color) {
	if n == nil {
		return
	}
	switch n.kind {
	case mathGroup:
		ui.Row(c).AlignItems(ui.Center).Gap(0).Children(func() {
			for i := 0; i < len(n.children); i++ {
				ch := n.children[i]
				if i > 0 && needsMathSpace(n.children[i-1], ch) {
					ui.Box(c).Width(4)
				}
				// Scale curly braces / parens cleanly when directly adjacent to a multi-row cases/array block.
				if ch.kind == mathOp && (ch.text == "{" || ch.text == "(" || ch.text == "[") && i+1 < len(n.children) && n.children[i+1].kind == mathCases && len(n.children[i+1].rows) > 1 {
					braceSize := size * min(float32(3.0), float32(len(n.children[i+1].rows))*0.95)
					ui.Text(c, ch.text).Font(mathSerifFont).FontSize(braceSize).TextColor(fg.Alpha(.85)).Padding(0, 2)
					continue
				}
				if ch.kind == mathOp && (ch.text == "}" || ch.text == ")" || ch.text == "]") && i > 0 && n.children[i-1].kind == mathCases && len(n.children[i-1].rows) > 1 {
					braceSize := size * min(float32(3.0), float32(len(n.children[i-1].rows))*0.95)
					ui.Text(c, ch.text).Font(mathSerifFont).FontSize(braceSize).TextColor(fg.Alpha(.85)).Padding(0, 2)
					continue
				}
				renderMathNodeUI(c, ch, size, fg)
			}
		})
	case mathFrac:
		if n.text == "binom" {
			ui.Column(c).Center().Gap(1).Children(func() {
				renderMathNodeUI(c, n.num, max(11, size-2), fg)
				renderMathNodeUI(c, n.den, max(11, size-2), fg)
			})
			return
		}
		ui.Column(c).Center().Gap(2).Padding(0, 4).Children(func() {
			renderMathNodeUI(c, n.num, max(11, size-2), fg)
			ui.Box(c).FillWidth().MinWidth(18).Height(1).Background(fg.Alpha(.75))
			renderMathNodeUI(c, n.den, max(11, size-2), fg)
		})
	case mathCases:
		ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
			if n.text == "cases" {
				braceSize := size * min(float32(3.0), max(float32(1.8), float32(len(n.rows))*0.95))
				ui.Text(c, "{").Font(mathSerifFont).FontSize(braceSize).TextColor(fg.Alpha(.85))
			}
			ui.Column(c).Gap(5).AlignItems(ui.Start).Children(func() {
				for _, row := range n.rows {
					ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
						for _, cell := range row {
							renderMathNodeUI(c, cell, size-0.5, fg)
						}
					})
				}
			})
		})
	case mathScript:
		// Big operators (\sum, \prod, \lim) stack limits above & below in display mode.
		if n.base != nil && (n.base.text == "∑" || n.base.text == "∏" || n.base.text == "lim") {
			ui.Column(c).Center().Gap(0).Padding(0, 4).Children(func() {
				if n.sup != nil {
					ui.Text(c, mathNodeUnicode(n.sup)).Font(mathSerifFont).Italic().FontSize(max(10, size-4)).TextColor(fg.Alpha(.88))
				}
				bigSize := size + 5
				if n.base.text == "lim" {
					bigSize = size
				}
				ui.Text(c, n.base.text).Font(mathSerifFont).FontSize(bigSize).TextColor(fg)
				if n.sub != nil {
					ui.Text(c, mathNodeUnicode(n.sub)).Font(mathSerifFont).Italic().FontSize(max(10, size-4)).TextColor(fg.Alpha(.88))
				}
			})
			return
		}
		ui.Row(c).AlignItems(ui.Center).Gap(0).Children(func() {
			renderMathNodeUI(c, n.base, size, fg)
			if n.sub != nil {
				ui.Text(c, mathNodeUnicode(n.sub)).Font(mathSerifFont).FontSize(max(10, size-4)).TextColor(fg).Margin(5, 0, 0, 0)
			}
			if n.sup != nil {
				ui.Text(c, mathNodeUnicode(n.sup)).Font(mathSerifFont).FontSize(max(10, size-4)).TextColor(fg).Margin(0, 0, 6, 0)
			}
		})
	default:
		txt := mathNodeUnicode(n)
		if txt != "" {
			el := ui.Text(c, txt).Font(mathSerifFont).FontSize(size).TextColor(fg)
			if n.kind == mathIdent {
				el.Italic()
			}
		}
	}
}
