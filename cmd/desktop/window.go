package main

import "github.com/egoist/mygo"

const (
	mainWinWidth, mainWinHeight       = 1360, 860
	mainWinMinWidth, mainWinMinHeight = 960, 680

	settingsWinWidth, settingsWinHeight       = 780, 540
	settingsWinMinWidth, settingsWinMinHeight = 620, 400

	loginPickerWidth, loginPickerHeight = 480, 308
	loginWebWidth, loginWebHeight       = 980, 760
	loginWebMinWidth, loginWebMinHeight = 780, 620
)

// centeredPosition computes the top-left origin that centers child within parent.
// When parent is invalid or smaller than child, it returns ok=false so callers
// can fall back to centering on the screen.
func centeredPosition(parent, child mygo.Rectangle) (x, y int, ok bool) {
	if parent.Width <= 0 || parent.Height <= 0 || child.Width <= 0 || child.Height <= 0 {
		return 0, 0, false
	}
	if child.Width > parent.Width || child.Height > parent.Height {
		return 0, 0, false
	}
	return parent.X + (parent.Width-child.Width)/2, parent.Y + (parent.Height-child.Height)/2, true
}

func centerWindowOver(child, parent *mygo.Window) {
	if child == nil || child.IsDestroyed() {
		return
	}
	if parent != nil && !parent.IsDestroyed() {
		if x, y, ok := centeredPosition(parent.Bounds(), child.Bounds()); ok {
			child.SetPosition(x, y)
			return
		}
	}
	child.Center()
}

// bringMainBehind raises the main Declic window right behind an active child window
// (such as Settings or Login) when the user switches back from another application,
// keeping the child window open and focused on top.
func (a *app) bringMainBehind(child *mygo.Window) {
	if a.closed || a.win == nil || child == nil || a.win.IsDestroyed() || child.IsDestroyed() || a.bringingMain {
		return
	}
	a.bringingMain = true
	defer func() { a.bringingMain = false }()
	wasMinimized := a.win.IsMinimized()
	if wasMinimized {
		a.win.Restore()
	}
	a.win.ShowInactive()
	if wasMinimized || !child.IsFocused() {
		child.Focus()
	}
}
