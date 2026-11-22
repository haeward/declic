package main

import (
	"runtime"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type dockIcon struct {
	id, label string
	png       []byte
}

func (d dockIcon) key() string {
	if d.id == "" {
		return "default"
	}
	return d.id
}

var dockIcons = [...]dockIcon{
	{"", "Default", ticketPreview},
	{"match", "Match", matchIcon},
	{"pancake", "Pancake", pancakeIcon},
	{"pruners", "Pruners", prunersIcon},
	{"nesting", "Nesting", nestingIcon},
}

func dockIconForID(id string) dockIcon {
	for _, icon := range dockIcons {
		if icon.id == id {
			return icon
		}
	}
	return dockIcons[0]
}

var loadDockIconPreviews = sync.OnceValues(func() ([len(dockIcons)]*ui.Bitmap, *ui.Bitmap) {
	var previews [len(dockIcons)]*ui.Bitmap
	for i, icon := range dockIcons {
		if len(icon.png) > 0 {
			previews[i], _ = ui.DecodeBitmap(icon.png)
		}
	}
	var darkTicket *ui.Bitmap
	if len(ticketDarkPreview) > 0 {
		darkTicket, _ = ui.DecodeBitmap(ticketDarkPreview)
	}
	return previews, darkTicket
})

func dockIconPreviews(dark bool) [len(dockIcons)]*ui.Bitmap {
	previews, darkTicket := loadDockIconPreviews()
	if dark && darkTicket != nil {
		previews[0] = darkTicket
	}
	return previews
}

func (a *app) applyDockIcon(icon dockIcon, setIcon func([]byte) error) bool {
	png := icon.png
	if icon.id == "" {
		// AppKit restores the bundle's appearance-aware icon when passed nil.
		png = nil
	} else if len(png) == 0 {
		a.dockIconError = "Could not change the Dock icon: icon asset unavailable"
		return false
	}
	if err := setIcon(png); err != nil {
		a.dockIconError = "Could not change the Dock icon: " + err.Error()
		return false
	}
	a.dockIconID, a.dockIconError = icon.id, ""
	return true
}

func (a *app) chooseDockIcon(icon dockIcon, setIcon func([]byte) error) {
	if a.dockIconID == icon.id && a.dockIconPref == icon.id && a.dockIconError == "" {
		return
	}
	if a.applyDockIcon(icon, setIcon) {
		a.dockIconPref = icon.id
	}
}

func (a *app) restoreDockIcon() {
	if runtime.GOOS == "darwin" {
		a.restoreDockIconWith(mygo.App.Dock.SetIcon)
	}
}

func (a *app) restoreDockIconWith(setIcon func([]byte) error) {
	if a.dockIconPref == "" {
		return
	}
	icon := dockIconForID(a.dockIconPref)
	a.dockIconID = "" // The bundle icon is visible until the override succeeds.
	a.applyDockIcon(icon, setIcon)
}

func (a *app) dockIconPicker(c *ui.Context, setIcon func([]byte) error) {
	t := c.Theme()
	previews := dockIconPreviews(t.Dark)
	ui.Column(c).FillWidth().Padding(8, 0).Gap(10).Children(func() {
		ui.Column(c).FillWidth().Gap(3).Children(func() {
			ui.Text(c, "Dock icon").FontSize(13).TextColor(t.Text)
			ui.Text(c, "Shown in the Dock while Declic is running.").FontSize(12).TextColor(t.TextMuted)
		})
		ui.RadioGroup(c, func() {
			for i, icon := range dockIcons {
				selected := a.dockIconID == icon.id
				button := ui.ButtonBase(c.Key("dock-"+icon.key())).
					Label(icon.label+" Dock icon").
					Role(ui.RoleRadio).
					Checked(selected).
					Width(64).Height(70).Radius(10).
					Border(1.5, ui.Transparent)
				fg, weight := t.TextMuted, 400
				if selected {
					button.Background(t.Selection).BorderColor(t.Accent)
					fg, weight = t.Text, 500
				} else if button.Pressed() {
					button.Background(t.SurfacePressed)
					fg = t.Text
				} else if button.Hovered() {
					button.Background(t.SurfaceHover)
					fg = t.Text
				}
				button.Children(func() {
					ui.Column(c).Fill().Center().Gap(3).Children(func() {
						if image := previews[i]; image != nil {
							ui.Image(c, image).Size(42, 42).Fit(ui.Contain)
						}
						ui.Text(c, icon.label).FontSize(11).FontWeight(weight).TextColor(fg).SingleLine()
					})
				}).OnClick(func() { a.chooseDockIcon(icon, setIcon) })
			}
		}).Row().Wrap().FillWidth().Gap(6).Label("Dock icon")
		if a.dockIconError != "" {
			ui.Text(c, a.dockIconError).FontSize(12).TextColor(t.Danger)
		}
	})
}
