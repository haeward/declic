package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/haeward/declic/internal/leetcode"
)

const sessionVerifyTTL = 5 * time.Minute

type verifiedSessionEntry struct {
	cookies    []*http.Cookie
	viewer     leetcode.Viewer
	verifiedAt time.Time
}

var (
	sessionCacheMu sync.Mutex
	sessionCache   = map[string]verifiedSessionEntry{}
)

func sessionCacheKey(site leetcode.Site, user string) string {
	return string(site) + "\x00" + user
}

func invalidateSessionCache(site leetcode.Site) {
	sessionCacheMu.Lock()
	for k := range sessionCache {
		if len(k) > len(site) && k[:len(site)] == string(site) && k[len(site)] == 0 {
			delete(sessionCache, k)
		}
	}
	sessionCacheMu.Unlock()
}

func siteCookies(site leetcode.Site) ([]*http.Cookie, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	session := mygo.NewSession(string(site))
	stored, err := readCookies(ctx, func() ([]mygo.Cookie, error) {
		return session.Cookies.List(mygo.CookieFilter{URL: site.URL() + "/graphql"})
	})
	if err != nil {
		return nil, err
	}
	var cookies []*http.Cookie
	for _, c := range stored {
		if c.Name == "LEETCODE_SESSION" || c.Name == "csrftoken" || c.Name == "cf_clearance" {
			cookies = append(cookies, &http.Cookie{Name: c.Name, Value: c.Value})
		}
	}
	return cookies, nil
}

// MyGo's native cookie API has no cancellation parameter. Bound our wait;
// a late native completion may only fill this channel, never update the UI.
func readCookies(ctx context.Context, read func() ([]mygo.Cookie, error)) ([]mygo.Cookie, error) {
	type result struct {
		cookies []mygo.Cookie
		err     error
	}
	done := make(chan result, 1)
	go func() {
		cookies, err := read()
		done <- result{cookies, err}
	}()
	select {
	case r := <-done:
		return r.cookies, r.err
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out reading site cookies; restart the app and try again (public browsing remains available): %w", ctx.Err())
	}
}

func (a *app) applyViewerProfile(v leetcode.Viewer) {
	a.premium = v.Premium
	if v.RealName != "" {
		a.nickname = v.RealName
	}
	if v.Avatar != "" && (v.Avatar != a.avatarURL || a.avatar == nil) {
		if v.Avatar != a.avatarURL {
			a.avatar = nil
		}
		a.avatarURL = v.Avatar
		avatarURL := v.Avatar
		user := v.Username
		if user == "" {
			user = a.username
		}
		site := a.site
		if a.post == nil {
			return
		}
		go func() {
			bmp, err := fetchSafeBitmap(avatarURL)
			if err != nil {
				return
			}
			a.post(func() {
				if a.closed || a.site != site || (a.username != "" && user != "" && a.username != user) || a.avatarURL != avatarURL {
					return
				}
				a.avatar = bmp
			})
		}()
	}
}

func (a *app) displayName() string {
	if a.nickname != "" {
		return a.nickname
	}
	return a.username
}

func (a *app) refreshSolved() {
	a.premium = false
	a.expired = false
	a.solved = nil
	a.statusFilter = ""
	a.solvedNotice = ""
	if a.username == "" || a.win == nil {
		return
	}
	site, user := a.site, a.username
	go func() {
		client := leetcode.New(site)
		cookies, viewer, err := verifiedCookies(context.Background(), client, user)
		var solved map[string]bool
		if err == nil {
			solved, err = client.Solved(context.Background(), cookies)
		}
		a.post(func() {
			if a.closed || a.site != site || a.username != user {
				return
			}
			if err != nil {
				a.solvedNotice = err.Error()
				if errors.Is(err, errSessionExpired) {
					a.expireSession()
				}
				return
			}
			a.applyViewerProfile(viewer)
			a.solved = solved
		})
	}()
}

var errSessionExpired = errors.New("session expired")

func verifiedCookies(ctx context.Context, client *leetcode.Client, user string) ([]*http.Cookie, leetcode.Viewer, error) {
	return verifiedCookiesWithCache(ctx, client, user, false)
}

func verifiedCookiesCached(ctx context.Context, client *leetcode.Client, user string) ([]*http.Cookie, leetcode.Viewer, error) {
	return verifiedCookiesWithCache(ctx, client, user, true)
}

func verifiedCookiesWithCache(ctx context.Context, client *leetcode.Client, user string, allowCache bool) ([]*http.Cookie, leetcode.Viewer, error) {
	key := sessionCacheKey(client.Site, user)
	if allowCache {
		sessionCacheMu.Lock()
		entry, ok := sessionCache[key]
		sessionCacheMu.Unlock()
		if ok && time.Since(entry.verifiedAt) < sessionVerifyTTL && len(entry.cookies) > 0 {
			return entry.cookies, entry.viewer, nil
		}
	}
	cookies, err := siteCookies(client.Site)
	var viewer leetcode.Viewer
	if err == nil {
		viewer, err = client.Viewer(ctx, cookies)
	}
	if err == nil && (!viewer.SignedIn || viewer.Username != user) {
		err = errSessionExpired
	}
	if err != nil {
		invalidateSessionCache(client.Site)
		return nil, leetcode.Viewer{}, err
	}
	sessionCacheMu.Lock()
	sessionCache[key] = verifiedSessionEntry{cookies: cookies, viewer: viewer, verifiedAt: time.Now()}
	sessionCacheMu.Unlock()
	return cookies, viewer, nil
}

// verifyStartupSession checks whether the selected site already has a saved session cookie
// and verifies it against userStatus. Guests with no LEETCODE_SESSION cookie make no request.
func verifyStartupSession(ctx context.Context, site leetcode.Site) (leetcode.Viewer, bool, error) {
	cookies, err := siteCookies(site)
	if err != nil {
		return leetcode.Viewer{}, false, err
	}
	hasSession := false
	for _, c := range cookies {
		if c.Name == "LEETCODE_SESSION" && c.Value != "" {
			hasSession = true
			break
		}
	}
	if !hasSession {
		return leetcode.Viewer{}, false, nil
	}
	viewer, err := leetcode.New(site).Viewer(ctx, cookies)
	if err != nil || !viewer.SignedIn || viewer.Username == "" {
		return leetcode.Viewer{}, false, err
	}
	sessionCacheMu.Lock()
	sessionCache[sessionCacheKey(site, viewer.Username)] = verifiedSessionEntry{
		cookies:    cookies,
		viewer:     viewer,
		verifiedAt: time.Now(),
	}
	sessionCacheMu.Unlock()
	return viewer, true, nil
}

func (a *app) expireSession() {
	invalidateSessionCache(a.site)
	a.expired = true
	a.premium = false
	a.solved = nil
	a.resources = nil
	a.resourceCache = nil
	a.purgePaidDetails()
	a.statusFilter = ""
	if a.catalog != nil {
		a.filterCatalog(a.search, a.skip)
	}
	a.status = a.username + " (" + string(a.site) + ") · Session expired"
	a.errorStatus = true
}
