package main

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Keep the operating system theme as the source when appearance is System.
func (a *app) applyTheme(c *ui.Context) {
	t := *c.Theme()
	if a.appearance == "Light" {
		t = *ui.LightTheme()
	}
	if a.appearance == "Dark" {
		t = *ui.DarkTheme()
	}
	t.Radius, t.FontSize = 8, 13
	if t.Dark {
		t.Background, t.Surface = ui.Hex("#0e0f11"), ui.Hex("#1a1b1f")
		t.Text, t.TextMuted, t.Border = ui.Hex("#e9e9ec"), ui.Hex("#8c8d95"), ui.Hex("#2e3037")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#24252a"), ui.Hex("#2e3038")
		t.Success, t.Warning, t.Danger = ui.Hex("#4cc38a"), ui.Hex("#ffca16"), ui.Hex("#ff7a70")
		t.Selection = ui.Hex("#1e2e4f")
		t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#2f6fe4"), ui.Hex("#3d7cf0"), ui.Hex("#275ec4")
	} else {
		t.Background, t.Surface = ui.Hex("#f1f2f6"), ui.Hex("#ffffff")
		t.Text, t.TextMuted, t.Border = ui.Hex("#1d1d1f"), ui.Hex("#6b6b73"), ui.Hex("#dcdce2")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#f4f5f9"), ui.Hex("#e6e8ef")
		t.Success, t.Warning, t.Danger = ui.Hex("#1a8f4c"), ui.Hex("#8a5200"), ui.Hex("#c62828")
		t.Selection = ui.Hex("#e4edff")
		t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#0a64f0"), ui.Hex("#0958d6"), ui.Hex("#084db8")
	}
	t.AccentText, t.Focus = ui.Hex("#ffffff"), ui.Hex("#8da4ef")
	t.TextMuted = t.TextMuted.Mix(t.Text, float32(a.contrast))
	t.Border = t.Border.Mix(t.TextMuted, float32(a.contrast))
	if c.Preferences().HighContrast {
		t.Border = t.TextMuted
		t.TextMuted = t.Text
	}
	c.SetTheme(&t)
}

func (a *app) syncNativeTheme() {
	source := mygo.ThemeSystem
	switch a.appearance {
	case "Light":
		source = mygo.ThemeLight
	case "Dark":
		source = mygo.ThemeDark
	}
	mygo.Theme.SetSource(source)
	if a.win != nil {
		a.win.Update(func() {})
	}
	if a.settingsWindow != nil {
		a.settingsWindow.Update(func() {})
	}
	if a.loginWindow != nil {
		a.loginWindow.Update(func() {})
	}
}

func sidebarColor(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#131417")
	}
	return ui.Hex("#eceef3")
}

func sidebarActiveColor(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#24252a")
	}
	return ui.Hex("#dcdee6")
}

func sidebarDividerColor(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#252730")
	}
	return ui.Hex("#d8dae3")
}

func settingsSidebarColor(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#131417")
	}
	return ui.Hex("#f2f3f7")
}

func chipBg(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#24252a")
	}
	return ui.Hex("#eeeff4")
}

func chipText(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#b8b9c0")
	}
	return ui.Hex("#45454c")
}

func activeChipBg(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#1d325a")
	}
	return ui.Hex("#e4edff")
}

func accentLinkColor(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#6ea6ff")
	}
	return ui.Hex("#0a4fc0")
}

func insetBg(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#141519")
	}
	return ui.Hex("#f5f5f8")
}

func hairlineColor(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.Hex("#24252b")
	}
	return ui.Hex("#ececf1")
}

func dailyCardColors(c *ui.Context) (bg, border ui.Color) {
	if c.Theme().Dark {
		return ui.Hex("#151c2a"), ui.Hex("#24334f")
	}
	return ui.Hex("#f5f8ff"), ui.Hex("#d9e1f7")
}

func dailyCardHoverColors(c *ui.Context) (bg, border ui.Color) {
	if c.Theme().Dark {
		return ui.Hex("#1a2438"), ui.Hex("#32476b")
	}
	return ui.Hex("#edf3ff"), ui.Hex("#c2d3f6")
}

func dailyTopicColors(c *ui.Context) (bg, border ui.Color) {
	if c.Theme().Dark {
		return ui.Hex("#1c263a"), ui.Hex("#2d3f60")
	}
	return ui.Hex("#ffffff"), ui.Hex("#d8e1f5")
}

func difficultyBadgeColors(c *ui.Context, d string) (fg, bg ui.Color) {
	dark := c.Theme().Dark
	switch strings.ToLower(d) {
	case "easy":
		if dark {
			return ui.Hex("#4cc38a"), ui.Hex("#12382d")
		}
		return ui.Hex("#0b7a6b"), ui.Hex("#e0f5f0")
	case "medium":
		if dark {
			return ui.Hex("#f5b942"), ui.Hex("#3a2b10")
		}
		return ui.Hex("#8a5200"), ui.Hex("#fff0d2")
	case "hard":
		if dark {
			return ui.Hex("#ff7a70"), ui.Hex("#3d1c1d")
		}
		return ui.Hex("#b3261e"), ui.Hex("#fde8e8")
	}
	return c.Theme().TextMuted, chipBg(c)
}

func solvedCircleColors(c *ui.Context) (fg, bg ui.Color) {
	if c.Theme().Dark {
		return ui.Hex("#4cc38a"), ui.Hex("#12382d")
	}
	return ui.Hex("#1a8f4c"), ui.Hex("#dff3e6")
}

func premiumBadgeColors(c *ui.Context) (fg, bg ui.Color) {
	if c.Theme().Dark {
		return ui.Hex("#b89dff"), ui.Hex("#2a2244")
	}
	return ui.Hex("#5b3fd0"), ui.Hex("#efeaff")
}

func tooltipColors(c *ui.Context) (bg, border, text, shadow ui.Color) {
	if c.Theme().Dark {
		return ui.Hex("#252730"), ui.Hex("#3a3d4a"), ui.Hex("#eceef2"), ui.RGBA(0, 0, 0, .42)
	}
	return ui.Hex("#ffffff"), ui.Hex("#d8dae2"), ui.Hex("#1d1d1f"), ui.RGBA(0, 0, 0, .14)
}
