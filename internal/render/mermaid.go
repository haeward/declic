package render

import (
	"math"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
)

type MermaidKind int

const (
	MermaidUnsupported MermaidKind = iota
	MermaidFlowchart
	MermaidSequence
	MermaidState
)

type MermaidNode struct {
	ID    string
	Label string
	Shape string // "rect", "round", "diamond", "circle", "dot"
	X, Y  float32
	W, H  float32
}

type MermaidEdge struct {
	From  string
	To    string
	Label string
	Arrow bool
}

type MermaidDiagram struct {
	Kind      MermaidKind
	KindLabel string
	Direction string // "TD", "LR", "RL", "BT"
	Nodes     []*MermaidNode
	NodeByID  map[string]*MermaidNode
	Edges     []MermaidEdge
	Actors    []string
	SeqSteps  []MermaidEdge
	CanvasW   float32
	CanvasH   float32
}

var (
	reFlowEdge = regexp.MustCompile(`^\s*(.+?)\s*(-->|<-->|-.->|==>|---|--\s+[^-]+?\s+-->)\s*(?:\|([^|]+)\|\s*)?(.+?)\s*$`)
	reSeqMsg   = regexp.MustCompile(`^\s*([A-Za-z0-9_\- ]+?)\s*(->>|-->>|->|-->|-x|--x)\s*([A-Za-z0-9_\- ]+?)\s*:\s*(.*)$`)
	reStateEdg = regexp.MustCompile(`^\s*(\[\*\]|[A-Za-z0-9_\-]+)\s*-->\s*(\[\*\]|[A-Za-z0-9_\-]+)(?:\s*:\s*(.*))?$`)
)

// ParseMermaid parses supported Mermaid diagram kinds (flowchart/graph, sequenceDiagram, stateDiagram).
func ParseMermaid(src string) *MermaidDiagram {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var clean []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		clean = append(clean, t)
	}
	if len(clean) == 0 {
		return &MermaidDiagram{Kind: MermaidUnsupported, KindLabel: "Mermaid"}
	}
	first := clean[0]
	low := strings.ToLower(first)
	switch {
	case strings.HasPrefix(low, "flowchart") || strings.HasPrefix(low, "graph"):
		dir := "TD"
		parts := strings.Fields(first)
		if len(parts) >= 2 {
			d := strings.ToUpper(parts[1])
			if d == "LR" || d == "RL" || d == "BT" || d == "TD" || d == "TB" {
				if d == "TB" {
					d = "TD"
				}
				dir = d
			}
		}
		return parseFlowchart(clean[1:], dir)
	case strings.HasPrefix(low, "sequencediagram"):
		return parseSequence(clean[1:])
	case strings.HasPrefix(low, "statediagram"):
		return parseStateDiagram(clean[1:])
	default:
		return &MermaidDiagram{Kind: MermaidUnsupported, KindLabel: "Mermaid"}
	}
}

func parseMermaidNodeToken(tok string) (id, label, shape string) {
	s := strings.TrimSpace(strings.TrimSuffix(tok, ";"))
	if s == "" {
		return "", "", "rect"
	}
	// Strip inline class suffix like :::foo
	if idx := strings.Index(s, ":::"); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	unquote := func(v string) string {
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"`)
		v = strings.ReplaceAll(v, "<br/>", " ")
		v = strings.ReplaceAll(v, "<br>", " ")
		return v
	}
	patterns := []struct {
		open, close, shape string
	}{
		{"((", "))", "circle"},
		{"([", "])", "round"},
		{"[", "]", "rect"},
		{"{", "}", "diamond"},
		{"(", ")", "round"},
	}
	for _, p := range patterns {
		if oi := strings.Index(s, p.open); oi > 0 && strings.HasSuffix(s, p.close) {
			id = strings.TrimSpace(s[:oi])
			inner := s[oi+len(p.open) : len(s)-len(p.close)]
			return id, unquote(inner), p.shape
		}
	}
	return s, unquote(s), "rect"
}

func addOrUpdateMermaidNode(d *MermaidDiagram, tok string) string {
	id, label, shape := parseMermaidNodeToken(tok)
	if id == "" {
		return ""
	}
	if existing, ok := d.NodeByID[id]; ok {
		if label != "" && label != id {
			existing.Label = label
		}
		if shape != "rect" || existing.Shape == "" {
			existing.Shape = shape
		}
		return id
	}
	if label == "" {
		label = id
	}
	n := &MermaidNode{ID: id, Label: label, Shape: shape}
	d.Nodes = append(d.Nodes, n)
	d.NodeByID[id] = n
	return id
}

func parseFlowchart(lines []string, dir string) *MermaidDiagram {
	d := &MermaidDiagram{
		Kind:      MermaidFlowchart,
		KindLabel: "Flowchart",
		Direction: dir,
		NodeByID:  map[string]*MermaidNode{},
	}
	for _, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, ";"))
		low := strings.ToLower(line)
		if line == "" || strings.HasPrefix(low, "subgraph") || low == "end" || strings.HasPrefix(low, "classdef ") || strings.HasPrefix(low, "class ") || strings.HasPrefix(low, "style ") || strings.HasPrefix(low, "linkstyle ") {
			continue
		}
		if m := reFlowEdge.FindStringSubmatch(line); len(m) == 5 {
			left, op, lbl, right := m[1], strings.TrimSpace(m[2]), strings.TrimSpace(m[3]), m[4]
			if lbl == "" && strings.HasPrefix(op, "-- ") && strings.HasSuffix(op, " -->") {
				lbl = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(op, "-- "), " -->"))
			}
			fromID := addOrUpdateMermaidNode(d, left)
			toID := addOrUpdateMermaidNode(d, right)
			if fromID != "" && toID != "" {
				d.Edges = append(d.Edges, MermaidEdge{
					From:  fromID,
					To:    toID,
					Label: lbl,
					Arrow: strings.Contains(op, ">"),
				})
			}
			continue
		}
		addOrUpdateMermaidNode(d, line)
	}
	if len(d.Nodes) == 0 {
		d.Kind = MermaidUnsupported
		return d
	}
	layoutLayeredGraph(d)
	return d
}

func parseStateDiagram(lines []string) *MermaidDiagram {
	d := &MermaidDiagram{
		Kind:      MermaidState,
		KindLabel: "State Diagram",
		Direction: "TD",
		NodeByID:  map[string]*MermaidNode{},
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "direction ") {
			if strings.Contains(strings.ToUpper(line), "LR") {
				d.Direction = "LR"
			}
			continue
		}
		if m := reStateEdg.FindStringSubmatch(line); len(m) == 4 {
			from, to, lbl := m[1], m[2], strings.TrimSpace(m[3])
			if from == "[*]" {
				from = "__start__"
				if _, ok := d.NodeByID[from]; !ok {
					n := &MermaidNode{ID: from, Label: "Start", Shape: "dot"}
					d.Nodes = append(d.Nodes, n)
					d.NodeByID[from] = n
				}
			} else {
				addOrUpdateMermaidNode(d, from+"("+from+")")
			}
			if to == "[*]" {
				to = "__end__"
				if _, ok := d.NodeByID[to]; !ok {
					n := &MermaidNode{ID: to, Label: "End", Shape: "dot"}
					d.Nodes = append(d.Nodes, n)
					d.NodeByID[to] = n
				}
			} else {
				addOrUpdateMermaidNode(d, to+"("+to+")")
			}
			d.Edges = append(d.Edges, MermaidEdge{From: from, To: to, Label: lbl, Arrow: true})
		}
	}
	if len(d.Nodes) == 0 {
		d.Kind = MermaidUnsupported
		return d
	}
	layoutLayeredGraph(d)
	return d
}

func parseSequence(lines []string) *MermaidDiagram {
	d := &MermaidDiagram{
		Kind:      MermaidSequence,
		KindLabel: "Sequence Diagram",
		NodeByID:  map[string]*MermaidNode{},
	}
	actorLabel := map[string]string{}
	addActor := func(id, label string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := actorLabel[id]; !ok {
			d.Actors = append(d.Actors, id)
			if label == "" {
				label = id
			}
			actorLabel[id] = label
		} else if label != "" {
			actorLabel[id] = label
		}
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "participant ") || strings.HasPrefix(low, "actor ") {
			rest := strings.TrimSpace(line[strings.IndexByte(line, ' ')+1:])
			if parts := strings.SplitN(rest, " as ", 2); len(parts) == 2 {
				addActor(parts[0], strings.TrimSpace(parts[1]))
			} else {
				addActor(rest, "")
			}
			continue
		}
		if m := reSeqMsg.FindStringSubmatch(line); len(m) == 5 {
			from, to, msg := strings.TrimSpace(m[1]), strings.TrimSpace(m[3]), strings.TrimSpace(m[4])
			addActor(from, "")
			addActor(to, "")
			d.SeqSteps = append(d.SeqSteps, MermaidEdge{
				From:  from,
				To:    to,
				Label: msg,
				Arrow: true,
			})
		}
	}
	if len(d.Actors) == 0 {
		d.Kind = MermaidUnsupported
		return d
	}
	const colW float32 = 132
	const padX float32 = 24
	const headerH float32 = 34
	const stepH float32 = 42
	for i, id := range d.Actors {
		w := max(float32(84), min(float32(124), float32(len([]rune(actorLabel[id])))*7.5+24))
		n := &MermaidNode{
			ID:    id,
			Label: actorLabel[id],
			Shape: "round",
			X:     padX + float32(i)*colW + (colW-w)/2,
			Y:     14,
			W:     w,
			H:     headerH,
		}
		d.Nodes = append(d.Nodes, n)
		d.NodeByID[id] = n
	}
	d.CanvasW = padX*2 + float32(len(d.Actors))*colW
	d.CanvasH = 14 + headerH + 22 + float32(max(1, len(d.SeqSteps)))*stepH + 18
	return d
}

func layoutLayeredGraph(d *MermaidDiagram) {
	inDeg := map[string]int{}
	adj := map[string][]string{}
	for _, n := range d.Nodes {
		inDeg[n.ID] = 0
	}
	for _, e := range d.Edges {
		if e.From == e.To {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
		inDeg[e.To]++
	}
	// Longest-path layering with cycle protection.
	layer := map[string]int{}
	var queue []string
	for _, n := range d.Nodes {
		if inDeg[n.ID] == 0 {
			queue = append(queue, n.ID)
			layer[n.ID] = 0
		}
	}
	if len(queue) == 0 && len(d.Nodes) > 0 {
		queue = append(queue, d.Nodes[0].ID)
		layer[d.Nodes[0].ID] = 0
	}
	visited := 0
	maxSteps := len(d.Nodes)*len(d.Nodes) + 16
	for len(queue) > 0 && visited < maxSteps {
		cur := queue[0]
		queue = queue[1:]
		visited++
		for _, nxt := range adj[cur] {
			if layer[nxt] < layer[cur]+1 && layer[cur]+1 < len(d.Nodes) {
				layer[nxt] = layer[cur] + 1
				queue = append(queue, nxt)
			}
		}
	}
	layers := map[int][]*MermaidNode{}
	maxLayer := 0
	for _, n := range d.Nodes {
		l := layer[n.ID]
		layers[l] = append(layers[l], n)
		if l > maxLayer {
			maxLayer = l
		}
		runeLen := float32(len([]rune(n.Label)))
		if n.Shape == "dot" {
			n.W, n.H = 22, 22
		} else {
			n.W = max(float32(72), min(float32(180), runeLen*7.2+28))
			n.H = 34
			if n.Shape == "diamond" {
				n.W = max(float32(96), n.W+26)
				n.H = 50
			}
		}
	}

	const pad float32 = 22
	const gapX float32 = 28
	const gapY float32 = 44

	if d.Direction == "LR" || d.Direction == "RL" {
		colX := pad
		var maxH float32
		colWidths := make([]float32, maxLayer+1)
		for l := 0; l <= maxLayer; l++ {
			for _, n := range layers[l] {
				if n.W > colWidths[l] {
					colWidths[l] = n.W
				}
			}
		}
		for idx := 0; idx <= maxLayer; idx++ {
			l := idx
			if d.Direction == "RL" {
				l = maxLayer - idx
			}
			nodes := layers[l]
			y := pad
			for _, n := range nodes {
				n.X = colX + (colWidths[l]-n.W)/2
				n.Y = y
				y += n.H + 20
			}
			if len(nodes) > 0 && y-20 > maxH {
				maxH = y - 20
			}
			colX += colWidths[l] + 44
		}
		// Center columns vertically.
		for l := 0; l <= maxLayer; l++ {
			nodes := layers[l]
			if len(nodes) == 0 {
				continue
			}
			used := nodes[len(nodes)-1].Y + nodes[len(nodes)-1].H - pad
			offset := (maxH - pad - used) / 2
			if offset > 0 {
				for _, n := range nodes {
					n.Y += offset
				}
			}
		}
		d.CanvasW = max(float32(280), colX-44+pad)
		d.CanvasH = max(float32(100), maxH+pad)
		return
	}

	// Vertical (TD / BT)
	var maxW float32
	rowWidths := make([]float32, maxLayer+1)
	rowHeights := make([]float32, maxLayer+1)
	for l := 0; l <= maxLayer; l++ {
		w := float32(0)
		for i, n := range layers[l] {
			if i > 0 {
				w += gapX
			}
			w += n.W
			if n.H > rowHeights[l] {
				rowHeights[l] = n.H
			}
		}
		rowWidths[l] = w
		if w > maxW {
			maxW = w
		}
	}
	totalW := max(float32(280), maxW+pad*2)
	y := pad
	for idx := 0; idx <= maxLayer; idx++ {
		l := idx
		if d.Direction == "BT" {
			l = maxLayer - idx
		}
		nodes := layers[l]
		x := (totalW - rowWidths[l]) / 2
		for _, n := range nodes {
			n.X = x
			n.Y = y + (rowHeights[l]-n.H)/2
			x += n.W + gapX
		}
		y += rowHeights[l] + gapY
	}
	d.CanvasW = totalW
	d.CanvasH = max(float32(100), y-gapY+pad)
}

// PaintMermaidCanvas draws a parsed Mermaid diagram directly on a MyGo GPU canvas.
func PaintMermaidCanvas(c *ui.Context, d *MermaidDiagram) {
	t := c.Theme()
	nodeFill := t.Surface
	nodeBorder := t.Border
	edgeColor := t.TextMuted.Alpha(.85)
	labelBg := ui.Hex("#f5f5f8")
	accent := ui.Hex("#0a4fc0")
	if t.Dark {
		labelBg = ui.Hex("#141519")
		accent = ui.Hex("#6ea6ff")
	}

	ui.Box(c).Size(d.CanvasW, d.CanvasH).Draw(func(p *ui.Painter, r ui.Rect) {
		ox, oy := r.X, r.Y
		if d.Kind == MermaidSequence {
			// Lifelines
			for _, n := range d.Nodes {
				cx := ox + n.X + n.W/2
				p.Line(cx, oy+n.Y+n.H, cx, oy+d.CanvasH-12, 1.2, nodeBorder)
			}
			// Messages
			stepY := oy + 14 + 34 + 30
			for _, step := range d.SeqSteps {
				from := d.NodeByID[step.From]
				to := d.NodeByID[step.To]
				if from == nil || to == nil {
					continue
				}
				x1 := ox + from.X + from.W/2
				x2 := ox + to.X + to.W/2
				if x1 == x2 {
					pts := [][2]float32{{x1, stepY - 8}, {x1 + 36, stepY - 8}, {x1 + 36, stepY + 10}, {x1, stepY + 10}}
					p.StrokePolyline(pts, 1.5, edgeColor)
					drawArrowHead(p, x1+36, stepY+10, x1, stepY+10, edgeColor)
					if step.Label != "" {
						p.Text(x1+42, stepY-7, step.Label, 11.5, t.TextMuted)
					}
				} else {
					p.StrokePolyline([][2]float32{{x1, stepY}, {x2, stepY}}, 1.5, edgeColor)
					drawArrowHead(p, x1, stepY, x2, stepY, edgeColor)
					if step.Label != "" {
						midX := (x1+x2)/2 - float32(len([]rune(step.Label)))*3.1
						p.Text(midX, stepY-16, step.Label, 11.5, t.TextMuted)
					}
				}
				stepY += 42
			}
		} else {
			// Edges
			for _, e := range d.Edges {
				from := d.NodeByID[e.From]
				to := d.NodeByID[e.To]
				if from == nil || to == nil {
					continue
				}
				x1, y1, x2, y2 := edgeEndpoints(from, to, d.Direction)
				x1 += ox
				y1 += oy
				x2 += ox
				y2 += oy
				p.StrokePolyline([][2]float32{{x1, y1}, {x2, y2}}, 1.5, edgeColor)
				if e.Arrow {
					drawArrowHead(p, x1, y1, x2, y2, edgeColor)
				}
				if e.Label != "" {
					lx := (x1 + x2) / 2
					ly := (y1 + y2) / 2
					lw := float32(len([]rune(e.Label)))*6.5 + 10
					p.Fill(ui.Rect{X: lx - lw/2, Y: ly - 9, W: lw, H: 18}, labelBg, 4)
					p.Text(lx-lw/2+5, ly-7, e.Label, 11, t.TextMuted)
				}
			}
		}
		// Nodes
		for _, n := range d.Nodes {
			box := ui.Rect{X: ox + n.X, Y: oy + n.Y, W: n.W, H: n.H}
			switch n.Shape {
			case "dot":
				p.Fill(box, accent, box.W/2)
			case "diamond":
				cx, cy := box.X+box.W/2, box.Y+box.H/2
				pts := [][2]float32{
					{cx, box.Y},
					{box.X + box.W, cy},
					{cx, box.Y + box.H},
					{box.X, cy},
					{cx, box.Y},
				}
				p.StrokePolyline(pts, 1.5, accent.Alpha(.8))
				tw := float32(len([]rune(n.Label))) * 6.4
				p.Text(max(box.X+14, cx-tw/2), cy-7.5, n.Label, 11.5, t.Text)
			case "circle":
				rad := min(box.W, box.H) / 2
				p.Fill(box, nodeFill, rad)
				p.Stroke(box, accent.Alpha(.75), rad, 1.3)
				tw := float32(len([]rune(n.Label))) * 6.8
				p.Text(max(box.X+8, box.X+(box.W-tw)/2), box.Y+(box.H-15)/2, n.Label, 12, t.Text)
			case "round":
				p.Fill(box, nodeFill, 14)
				p.Stroke(box, nodeBorder, 14, 1.2)
				tw := float32(len([]rune(n.Label))) * 6.8
				p.Text(max(box.X+8, box.X+(box.W-tw)/2), box.Y+(box.H-15)/2, n.Label, 12, t.Text)
			default:
				p.Fill(box, nodeFill, 8)
				p.Stroke(box, nodeBorder, 8, 1.2)
				tw := float32(len([]rune(n.Label))) * 6.8
				p.Text(max(box.X+8, box.X+(box.W-tw)/2), box.Y+(box.H-15)/2, n.Label, 12, t.Text)
			}
		}
	})
}

func edgeEndpoints(from, to *MermaidNode, dir string) (x1, y1, x2, y2 float32) {
	if dir == "LR" || dir == "RL" {
		if from.X <= to.X {
			return from.X + from.W, from.Y + from.H/2, to.X, to.Y + to.H/2
		}
		return from.X, from.Y + from.H/2, to.X + to.W, to.Y + to.H/2
	}
	if from.Y <= to.Y {
		return from.X + from.W/2, from.Y + from.H, to.X + to.W/2, to.Y
	}
	return from.X + from.W/2, from.Y, to.X + to.W/2, to.Y + to.H
}

func drawArrowHead(p *ui.Painter, x1, y1, x2, y2 float32, color ui.Color) {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	ang := math.Atan2(dy, dx)
	const headLen = 7.0
	const spread = 0.45
	lx := x2 - float32(headLen*math.Cos(ang-spread))
	ly := y2 - float32(headLen*math.Sin(ang-spread))
	rx := x2 - float32(headLen*math.Cos(ang+spread))
	ry := y2 - float32(headLen*math.Sin(ang+spread))
	p.StrokePolyline([][2]float32{{lx, ly}, {x2, y2}, {rx, ry}}, 1.5, color)
}
