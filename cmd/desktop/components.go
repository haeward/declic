package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/egoist/mygo/ui"
)

var smoothColorTransition = ui.ElementTransition{Duration: 130 * time.Millisecond, Colors: true}

func insetHairline(c *ui.Context, color ui.Color, top, horiz, bottom float32) {
	ui.Box(c).FillWidth().Padding(top, horiz, bottom, horiz).Children(func() {
		ui.Box(c).FillWidth().Height(1).Background(color)
	})
}

func applyPressFeedback(el ui.Element, pressedScale float32) ui.Element {
	target := float32(1)
	if el.Pressed() {
		target = pressedScale
	}
	s := el.AnimateTo("press", target, ui.Snappy)
	if s != 1 {
		el.Scale(s, s)
	}
	return el.Transition(smoothColorTransition)
}

func quietButton(c *ui.Context, label string) ui.Element {
	b := ui.ButtonBase(c).Label(label).Height(30).Padding(0, 10).Radius(8).AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	b.Children(func() { ui.Text(c, label).FontSize(12.5).SingleLine() })
	return b
}

func linkButton(c *ui.Context, label string) ui.Element {
	b := ui.ButtonBase(c).Label(label).Height(28).Padding(0, 8).Radius(7).AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	b.Children(func() { ui.Text(c, label).FontSize(12.5).TextColor(accentLinkColor(c)).SingleLine() })
	return b
}

func outlineButton(c *ui.Context, label string, icon *ui.SVG, height float32) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Height(height).Padding(0, 14).Radius(9).Border(1, t.Border).Background(t.Surface).AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	applyPressFeedback(b, 0.975)
	b.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if icon != nil {
				ui.Icon(c, icon).FontSize(14).TextColor(t.TextMuted)
			}
			ui.Text(c, label).FontSize(13).TextColor(t.Text).SingleLine()
		})
	})
	return b
}

func primaryActionButton(c *ui.Context, label string, icon *ui.SVG, height float32) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Height(height).Padding(0, 16).Radius(9).Border(1, t.Accent).Background(t.Accent).AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(t.AccentHover).BorderColor(t.AccentHover)
	}
	applyPressFeedback(b, 0.975)
	b.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if icon != nil {
				ui.Icon(c, icon).FontSize(14).TextColor(t.AccentText)
			}
			ui.Text(c, label).FontSize(13).FontWeight(600).TextColor(t.AccentText).SingleLine()
		})
	})
	return b
}

type tipSide int

const (
	tipBottom tipSide = iota
	tipTop
	tipRight
	tipLeft
)

func attachTooltip(c *ui.Context, anchor ui.Element, text string, side tipSide) ui.Element {
	if text == "" {
		return anchor
	}
	bg, border, fg, shadow := tooltipColors(c)
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		switch side {
		case tipRight:
			tip.AttachTo(anchor, ui.AnchorEnd, ui.AnchorStart).Margin(0, 0, 0, 8)
		case tipLeft:
			tip.AttachTo(anchor, ui.AnchorStart, ui.AnchorEnd).Margin(0, 8, 0, 0)
		case tipTop:
			tip.AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, 6, 0)
		default:
			tip.AttachTo(anchor, ui.AnchorBottom, ui.AnchorTop).Margin(6, 0, 0, 0)
		}
		tip.MaxWidth(240).Padding(4, 9).Radius(7).Background(bg).Border(1, border).Shadow(0, 4, 14, 0, shadow).Children(func() {
			ui.Text(c, text).FontSize(11.5).FontWeight(500).TextColor(fg)
		})
	})
	return anchor
}

func iconActionButtonTip(c *ui.Context, label, tooltip string, icon *ui.SVG, fg, hoverBg ui.Color, side tipSide) ui.Element {
	b := ui.ButtonBase(c).Label(label).Size(28, 28).Radius(7).Center()
	iconFg := fg
	if b.Hovered() {
		b.Background(hoverBg)
		if hoverBg == c.Theme().SurfaceHover {
			iconFg = c.Theme().Text
		}
	}
	applyPressFeedback(b, 0.94)
	b.Children(func() { ui.Icon(c, icon).FontSize(15).TextColor(iconFg) })
	return attachTooltip(c, b, tooltip, side)
}

func iconToolbarButton(c *ui.Context, label, tooltip string, icon *ui.SVG, active bool, badge int) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Size(38, 38).Radius(10).Border(1, t.Border).Background(t.Surface).Center()
	fg := t.TextMuted
	if active {
		b.Background(activeChipBg(c)).BorderColor(t.Accent.Alpha(.45))
		fg = accentLinkColor(c)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
		fg = t.Text
	}
	applyPressFeedback(b, 0.95)
	b.Children(func() {
		ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon).FontSize(15).TextColor(fg)
			if badge > 0 {
				ui.Text(c, fmt.Sprint(badge)).FontSize(10.5).FontWeight(700).TextColor(accentLinkColor(c))
			}
		})
	})
	return attachTooltip(c, b, tooltip, tipBottom)
}

func titleBarIconButton(c *ui.Context, label, tooltip string, icon *ui.SVG, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Size(28, 28).Radius(7).Center().Disabled(disabled)
	fg := t.TextMuted
	if disabled {
		fg = t.TextMuted.Alpha(.38)
	} else if b.Hovered() {
		b.Background(sidebarActiveColor(c).Alpha(.7))
		fg = t.Text
	}
	if !disabled {
		applyPressFeedback(b, 0.94)
	}
	b.Children(func() { ui.Icon(c, icon).FontSize(15).TextColor(fg) })
	return attachTooltip(c, b, tooltip, tipBottom)
}

func segmentedTrack(c *ui.Context, fn func()) ui.Element {
	return ui.Row(c).Padding(3).Gap(2).Radius(10).Background(chipBg(c)).AlignItems(ui.Center).Children(fn)
}

func segmentItem(c *ui.Context, label string, selected bool, fg ui.Color, height float32) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Height(height).Padding(0, 12).Radius(7).AlignItems(ui.Center)
	if selected {
		b.Background(t.Surface).Border(1, t.Border.Alpha(.55))
	} else if b.Hovered() {
		b.Background(t.Surface.Alpha(.45))
	}
	applyPressFeedback(b, 0.97)
	b.Children(func() { ui.Text(c, label).FontSize(12.5).FontWeight(600).TextColor(fg).SingleLine() })
	return b
}

func chipButton(c *ui.Context, label string, active bool) ui.Element {
	bg, fg := chipBg(c), chipText(c)
	weight := 400
	if active {
		bg, fg = activeChipBg(c), accentLinkColor(c)
		weight = 600
	}
	b := ui.ButtonBase(c).Label(label).Height(26).Padding(0, 11).Radius(13).Background(bg).AlignItems(ui.Center)
	if b.Hovered() && !active {
		b.Background(c.Theme().SurfacePressed)
	}
	applyPressFeedback(b, 0.96)
	b.Children(func() { ui.Text(c, label).FontSize(12).FontWeight(weight).TextColor(fg).SingleLine() })
	return b
}

func solvedCheckBadge(c *ui.Context) {
	fg, bg := solvedCircleColors(c)
	ui.Box(c).Size(18, 18).Radius(9).Background(bg).Center().Children(func() {
		ui.Icon(c, iconCheck).FontSize(11).TextColor(fg)
	})
}

func navItem(c *ui.Context, icon *ui.SVG, label string, selected bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).FillWidth().Height(34).Padding(0, 10).Radius(8)
	if selected {
		b.Background(sidebarActiveColor(c))
	} else if b.Hovered() {
		b.Background(sidebarActiveColor(c).Alpha(.55))
	}
	applyPressFeedback(b, 0.985)
	b.Children(func() {
		ui.Row(c).Fill().Gap(10).AlignItems(ui.Center).Children(func() {
			fg := t.TextMuted
			weight := 400
			if selected {
				fg = t.Text
				weight = 600
			}
			ui.Box(c).Width(22).Center().Shrink(0).Children(func() {
				ui.Icon(c, icon).FontSize(15.5).TextColor(fg)
			})
			ui.Text(c, label).FontSize(13).FontWeight(weight).TextColor(fg).Grow(1).SingleLine()
		})
	})
	return b
}

func tabButton(c *ui.Context, label string, selected bool) ui.Element {
	return tabButtonWithIcon(c, nil, label, selected, c.Theme().Accent)
}

func tabButtonWithIcon(c *ui.Context, icon *ui.SVG, label string, selected bool, iconTint ui.Color) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Height(44).Padding(0, 2).AlignItems(ui.Center)
	color := t.TextMuted
	weight := 400
	iconColor := t.TextMuted.Alpha(.85)
	if selected {
		b.BorderWidth(0, 0, 2, 0).BorderColor(t.Accent)
		color = t.Text
		weight = 600
		iconColor = iconTint
	}
	b.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if icon != nil {
				ui.Icon(c, icon).FontSize(14).TextColor(iconColor)
			}
			ui.Text(c, label).FontSize(13).FontWeight(weight).TextColor(color).SingleLine()
		})
	})
	return b
}

func pillTabButton(c *ui.Context, label string, selected bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Height(28).Padding(0, 10).Radius(7).AlignItems(ui.Center)
	color := t.TextMuted
	weight := 500
	if selected {
		b.Background(chipBg(c))
		color = t.Text
		weight = 600
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Text(c, label).FontSize(12.5).FontWeight(weight).TextColor(color).SingleLine() })
	return b
}

func nullEmptyState(c *ui.Context, title, detail string) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().MinHeight(420).Center().Padding(32).Gap(10).Children(func() {
		ui.Icon(c, iconNullArt).Size(120, 82)
		if title != "" {
			ui.Text(c, title).FontSize(13.5).FontWeight(500).TextColor(t.TextMuted).TextAlign(ui.Center)
		}
		if detail != "" {
			ui.Text(c, detail).MaxWidth(320).FontSize(12.5).TextColor(t.TextMuted.Alpha(.85)).TextAlign(ui.Center)
		}
	})
}

func emptyStateAction(c *ui.Context, icon *ui.SVG, title, detail, actionLabel string, onClick func()) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Center().Padding(32).Gap(12).Children(func() {
		ui.Box(c).Size(48, 48).Radius(24).Background(chipBg(c)).Center().Children(func() {
			ui.Icon(c, icon).FontSize(22).TextColor(t.TextMuted)
		})
		ui.Column(c).Center().Gap(6).Children(func() {
			ui.Text(c, title).FontSize(16).Bold().TextColor(t.Text)
			if detail != "" {
				ui.Text(c, detail).MaxWidth(340).FontSize(13).TextAlign(ui.Center).TextColor(t.TextMuted)
			}
		})
		if actionLabel != "" && onClick != nil {
			ui.Box(c).Height(4)
			primaryActionButton(c, actionLabel, nil, 36).OnClick(onClick)
		}
	})
}

func (a *app) statusView(c *ui.Context) {
	ui.Row(c).FillWidth().Padding(8, 16).Gap(12).Background(c.Theme().Danger.Alpha(.08)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.status).FontSize(12).TextColor(c.Theme().Danger).Grow(1)
		if a.expired {
			quietButton(c, "Sign in again").OnClick(a.openLogin)
		}
		if a.restricted.Slug != "" {
			ui.Link(c, "Open on "+string(a.site)+" ↗", a.site.URL()+"/problems/"+url.PathEscape(a.restricted.Slug)+"/")
		}
		if a.saveFailed {
			quietButton(c, "Retry save").Disabled(a.busy).OnClick(func() { a.save(nil) })
		}
		quietButton(c, "Dismiss").OnClick(func() { a.errorStatus = false; a.restricted.Slug = "" })
	})
}
