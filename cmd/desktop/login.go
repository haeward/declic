package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
)

func isPostLoginSiteURL(site leetcode.Site, raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	if !strings.EqualFold(u.Hostname(), string(site)) {
		return false
	}
	return !strings.HasPrefix(u.Path, "/accounts/login")
}

func (a *app) openLogin() {
	if a.loginWindow != nil {
		a.loginWindow.Focus()
		return
	}
	a.loginSite = a.site
	a.accountNotice = ""
	a.loginAutoChecking = false
	a.loginLoading = false
	a.loginLoadError = ""
	if a.win == nil {
		return
	}
	parent := a.win
	if a.settingsWindow != nil && a.settingsWindow.IsFocused() {
		parent = a.settingsWindow
	}
	var w *mygo.Window
	w = mygo.NewWindow(mygo.WindowOptions{
		TitleBarStyle:     mygo.TitleBarHidden,
		HideWindowButtons: true,
		DisableResize:     true,
		Width:             loginPickerWidth,
		Height:            loginPickerHeight,
		Parent:            parent,
		Content:           ui.View(a.loginPanel),
	})
	centerWindowOver(w, parent)
	a.loginWindow = w
	w.OnFocus(func() { a.bringMainBehind(w) })
	w.OnClosed(func() {
		if a.loginWindow == w {
			a.loginWindow = nil
			a.loginView = nil
			a.loginAutoChecking = false
		}
	})
}

func (a *app) loginPanelCloseRequested(c *ui.Context) bool {
	// Only allow Escape to dismiss the borderless site-picker dialog; inside the
	// official login WebView, unhandled Escape keys from page controls bubble up
	// in MyGo v0.10+ and must not close an in-progress sign-in flow.
	return (a.loginView == nil && c.Shortcut(0, ui.KeyEscape)) || primaryShortcut(c, ui.KeyW)
}

func (a *app) dismissLoginWindow() {
	if a.closeLogin != nil {
		a.closeLogin()
		return
	}
	if a.loginWindow != nil {
		a.loginWindow.Close()
	}
}

func (a *app) loginPanel(c *ui.Context) {
	a.applyTheme(c)
	t := c.Theme()
	if a.loginPanelCloseRequested(c) && (a.loginWindow != nil || a.closeLogin != nil) {
		a.dismissLoginWindow()
		return
	}
	if a.loginView == nil {
		ui.Column(c).Fill().Padding(24, 26, 22, 26).Gap(16).Background(t.Surface).DragWindow().Children(func() {
			ui.Column(c).FillWidth().Gap(5).Children(func() {
				ui.Text(c, "Sign in to LeetCode").FontSize(17).Bold().TextColor(t.Text)
				ui.Text(c, "Choose the site your account belongs to.").FontSize(13).TextColor(t.TextMuted)
			})
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				for _, site := range []leetcode.Site{leetcode.COM, leetcode.CN} {
					name := "LeetCode"
					if site == leetcode.CN {
						name = "LeetCode CN"
					}
					selected := site == a.loginSite
					b := ui.ButtonBase(c).Label(name).Grow(1).Width(0).Border(1.5, ui.Transparent).Padding(15, 16).Radius(12).Background(insetBg(c))
					if selected {
						b.Background(activeChipBg(c)).Border(1.5, t.Accent)
					} else if b.Hovered() {
						b.Background(t.SurfaceHover)
					}
					b.Children(func() {
						ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
							radio := ui.Box(c).Size(17, 17).Radius(8.5).Shrink(0)
							if selected {
								radio.Background(t.Surface).Border(5, t.Accent)
							} else {
								radio.Border(1.5, t.TextMuted.Alpha(.7))
							}
							ui.Column(c).Gap(3).AlignItems(ui.Start).Children(func() {
								ui.Text(c, name).FontSize(13.5).FontWeight(600).TextColor(t.Text)
								ui.Text(c, string(site)).FontSize(12).TextColor(t.TextMuted)
							})
						})
					})
					b.OnClick(func() { a.loginSite = site })
				}
			})
			ui.Text(c, "Next you sign in on the official page inside an isolated session. Declic never sees your password.").FontSize(12).TextColor(t.TextMuted)
			if a.accountNotice != "" {
				ui.Text(c, a.accountNotice).FontSize(12).TextColor(t.Danger)
			}
			ui.Box(c).Grow(1)
			ui.Row(c).FillWidth().Justify(ui.End).Gap(10).AlignItems(ui.Center).Children(func() {
				outlineButton(c, "Cancel", nil, 36).OnClick(func() {
					if a.loginWindow != nil {
						a.loginWindow.Close()
					}
				})
				primaryActionButton(c, "Continue", nil, 36).OnClick(a.startLogin)
			})
		})
		return
	}

	// Clean full-bleed official login WebView; show native loading or error state until DOM is ready.
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		if a.loginLoadError != "" {
			ui.Column(c).Grow(1).FillWidth().Center().Padding(32).Gap(12).Children(func() {
				ui.Box(c).Size(48, 48).Radius(24).Background(chipBg(c)).Center().Children(func() {
					ui.Icon(c, iconGlobe).FontSize(22).TextColor(t.TextMuted)
				})
				ui.Text(c, "Could not load sign-in page").FontSize(16).Bold().TextColor(t.Text)
				ui.Text(c, a.loginLoadError).MaxWidth(380).FontSize(13).TextAlign(ui.Center).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Padding(4, 0, 0, 0).Children(func() {
					primaryActionButton(c, "Retry", iconReset, 34).OnClick(func() {
						a.loginLoadError = ""
						a.loginLoading = true
						if a.loginView != nil {
							_ = a.loginView.Page().LoadURL(a.loginSite.URL() + "/accounts/login/")
						}
					})
				})
			})
		} else if a.loginLoading {
			ui.Column(c).Grow(1).FillWidth().Center().Padding(32).Gap(12).Children(func() {
				ui.Box(c).Size(48, 48).Radius(24).Background(activeChipBg(c)).Center().Children(func() {
					ui.Icon(c, iconGlobe).FontSize(22).TextColor(accentLinkColor(c))
				})
				ui.Text(c, "Loading "+string(a.loginSite)+"…").FontSize(15.5).FontWeight(600).TextColor(t.Text)
				ui.Text(c, "Connecting to the official sign-in page in an isolated session.").FontSize(12.5).TextColor(t.TextMuted)
			})
		} else {
			ui.WebView(c, a.loginView).Grow(1).FillWidth()
		}
		if a.accountNotice != "" {
			ui.Row(c).FillWidth().Height(50).Shrink(0).Padding(0, 16).Gap(10).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(hairlineColor(c)).Background(t.Surface).Children(func() {
				ui.Text(c, a.accountNotice).FontSize(12).TextColor(t.Danger).Grow(1).SingleLine()
				outlineButton(c, "Verify sign-in", nil, 30).Disabled(a.busy).OnClick(a.verifyLogin)
			})
		}
	})
}

func (a *app) startLogin() {
	site := a.loginSite
	old := a.loginWindow
	parent := a.win
	if a.settingsWindow != nil && a.settingsWindow.IsFocused() {
		parent = a.settingsWindow
	}
	var w *mygo.Window
	w = mygo.NewWindow(mygo.WindowOptions{
		Title:     "Sign in · " + string(site),
		Width:     loginWebWidth,
		Height:    loginWebHeight,
		MinWidth:  loginWebMinWidth,
		MinHeight: loginWebMinHeight,
		Parent:    parent,
		Content:   ui.View(a.loginPanel),
	})
	centerWindowOver(w, a.win)
	w.OnFocus(func() { a.bringMainBehind(w) })
	v, err := w.NewWebView(mygo.WebViewOptions{
		URL:  site.URL() + "/accounts/login/",
		Page: mygo.PageOptions{Session: mygo.NewSession(string(site)), Untrusted: true},
	})
	if err != nil {
		w.Close()
		a.accountNotice = err.Error()
		if old != nil {
			old.Update(func() {})
		}
		return
	}
	v.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
		if !strings.HasPrefix(e.URL, "https://") {
			e.PreventDefault()
		}
	})
	lastURL := ""
	maybeVerify := func(currentURL string) {
		if currentURL != "" {
			lastURL = currentURL
		}
		if !isPostLoginSiteURL(site, lastURL) {
			return
		}
		a.post(func() { a.autoVerifyLogin(site, w) })
	}
	markReady := func() {
		a.post(func() {
			if a.closed || a.loginWindow != w {
				return
			}
			if a.loginLoading || a.loginLoadError != "" {
				a.loginLoading = false
				a.loginLoadError = ""
				w.Update(func() {})
			}
		})
	}
	v.Page().OnDOMReady(markReady)
	v.Page().OnDidNavigate(func(u string) { maybeVerify(u) })
	v.Page().OnDidFinishLoad(func() {
		markReady()
		maybeVerify("")
	})
	v.Page().OnDidFailLoad(func(e *mygo.LoadError) {
		if e == nil || e.Code == -999 {
			return
		}
		a.post(func() {
			if a.closed || a.loginWindow != w {
				return
			}
			a.loginLoading = false
			a.loginLoadError = e.Error()
			w.Update(func() {})
		})
	})

	done := make(chan struct{})
	a.accountNotice = ""
	a.loginAutoChecking = false
	a.loginLoading = true
	a.loginLoadError = ""
	a.loginWindow = w
	a.loginView = v
	w.OnClosed(func() {
		close(done)
		if a.loginWindow == w {
			a.loginWindow = nil
			a.loginView = nil
			a.loginAutoChecking = false
			a.loginLoading = false
			a.loginLoadError = ""
		}
	})
	// Poll session cookies gently while the login window is open so SPA logins auto-complete.
	go func() {
		ticker := time.NewTicker(1800 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				a.post(func() { a.autoVerifyLogin(site, w) })
			}
		}
	}()
	if old != nil {
		old.Close()
	}
	w.Update(func() {})
	w.Focus()
}

func (a *app) autoVerifyLogin(site leetcode.Site, w *mygo.Window) {
	if a.closed || a.busy || a.loginAutoChecking || a.loginWindow != w {
		return
	}
	a.loginAutoChecking = true
	go func() {
		cookies, err := siteCookies(site)
		var viewer leetcode.Viewer
		if err == nil && len(cookies) > 0 {
			viewer, err = leetcode.New(site).Viewer(context.Background(), cookies)
		}
		a.post(func() {
			a.loginAutoChecking = false
			if a.closed || a.loginWindow != w {
				return
			}
			if err == nil && viewer.SignedIn && viewer.Username != "" {
				a.save(func() {
					if a.loginWindow != w {
						return
					}
					a.commitLogin(site, w, viewer)
				})
			}
		})
	}()
}

func (a *app) commitLogin(site leetcode.Site, w *mygo.Window, viewer leetcode.Viewer) {
	invalidateSessionCache(site)
	account := fmt.Sprintf("%x", sha256.Sum256([]byte(viewer.Username)))
	w.Close()
	if site != a.site {
		a.resetDocument(site)
		a.selectIdentity(account, viewer.Username)
		a.applyViewerProfile(viewer)
		a.list("", 0)
	} else {
		a.selectIdentity(account, viewer.Username)
		a.applyViewerProfile(viewer)
		if a.expired {
			a.refreshSolved()
		}
	}
}

func (a *app) verifyLogin() {
	site, w := a.loginSite, a.loginWindow
	a.save(func() {
		if a.loginWindow != w {
			return
		}
		a.job(func() (func(), error) {
			cookies, err := siteCookies(site)
			var viewer leetcode.Viewer
			if err == nil {
				viewer, err = leetcode.New(site).Viewer(context.Background(), cookies)
			}
			return func() {
				if a.loginWindow != w {
					return
				}
				if err != nil || !viewer.SignedIn || viewer.Username == "" {
					a.accountNotice = "Complete official sign-in and retry verification."
					if err != nil {
						a.accountNotice = err.Error()
					}
					w.Update(func() {})
					return
				}
				a.commitLogin(site, w, viewer)
			}, nil
		})
	})
}
