package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/render"
	"github.com/haeward/declic/internal/workspace"
)

type navEntry struct {
	screen string
	slug   string
}

type app struct {
	navHistory                    []navEntry
	navIndex                      int
	navReplaying                  bool
	accessFilter                  string
	sortOpen, langOpen            bool
	sitePopup, editorSettingsOpen bool
	selectedRow                   int
	listKeyboardNav               bool
	hoverRowPlus1                 int
	lastCatalogScrollY            float32
	catalogSlotH                  float32
	catalogViewportH              float32
	codeCopiedUntil               time.Time
	codeFormattedUntil            time.Time
	codeRetrievedUntil            time.Time
	retrievingSub                 bool
	subUsedUntil                  time.Time
	loginAutoChecking             bool
	loginLoading                  bool
	loginLoadError                string
	nickname, avatarURL           string
	avatar                        *ui.Bitmap
	recentProblems                []leetcode.Problem
	detailCache                   map[string]leetcode.Problem
	resourceCache                 map[string]map[string]*resourcePage
	playgrounds                   map[string]*playgroundState
	codeTabSelected               map[string]string
	slideDecks                    map[string]*slideDeckState
	linkedImages                  map[string]*linkedImageState
	mermaidShowSrc                map[string]bool
	videoSession                  *mygo.Session
	inlineVideos                  map[string]*mygo.WebView
	editorMaximized               bool
	loadingProblem                bool
	premiumModal                  bool
	mdOpts                        *markdown.Options
	mdCodeOpts                    *markdown.Options
	catalogRetry                  time.Time
	premium, expired              bool
	restricted                    leetcode.Problem
	focusCode                     bool
	editorSplit                   float32
	solved                        map[string]bool
	statusFilter, solvedNotice    string
	queryAt                       time.Time
	pendingQuery                  string
	resources                     map[string]*resourcePage
	quickListed                   string
	catalogCancel                 context.CancelFunc
	catalogGeneration             uint64
	catalogLoading                bool
	lastDailyCheck                string
	judge                         *judgeState
	submitAfter                   time.Time
	contrast                      float64
	resourceTab                   string
	resultTab                     string
	testPanelCollapsed            bool
	settingsSection               string
	dockIconID, dockIconPref      string
	dockIconError                 string
	sidebarCollapsed              bool
	accountPopup                  bool
	loginSite                     leetcode.Site
	loginView                     *mygo.WebView
	loginWindow, settingsWindow   *mygo.Window
	catalog                       []leetcode.Problem
	matches                       []leetcode.Problem
	difficulties, tags            map[string]bool
	filterOpen, hidePremium       bool
	tagSearch, sortColumn         string
	descending                    bool
	pageSize                      int
	dailyCollapsed                bool
	testcases                     []string
	caseIndex                     int
	caseChipSelected              bool
	showTopics                    bool
	compact                       bool
	descriptionSize               int
	editorFontSize, editorTabSize int
	hideAcceptance                bool
	hideDifficulty                bool
	catalogScroll                 ui.ScrollState
	showQuickOpen                 bool
	quickFocusPending             bool
	quickSearch                   string
	quickPage                     leetcode.Page
	quickError                    string

	win              *mygo.Window
	post             func(func())
	preview          *mygo.WebView
	store            workspace.Store
	site             leetcode.Site
	account          string
	search           string
	listed           string
	page             leetcode.Page
	skip             int
	problem          leetcode.Problem
	language         string
	code             *editor.State
	busy             bool
	saving           bool // Snapshot-only saves leave the editor and IME active.
	closed           bool
	status           string
	split            float32
	changed          time.Time
	version          uint64
	screen           string
	username         string
	accountNotice    string
	daily            leetcode.Daily
	dailyError       string
	catalogError     string
	saveFailed       bool
	draftSaved       bool
	errorStatus      bool
	appearance       string
	restoreTemplate  bool
	english          bool
	narrowPane       string
	prefPath         string
	prefReadErr      error
	lastSavedPrefs   preferences
	prefDirty        bool
	prefDirtyAt      time.Time
	prefSaving       bool
	openOnLaunch     string
	lastProblemSlug  string
	noRestoreSession bool
	storageLoaded    bool
	storageScanning  bool
	storageClearing  bool
	storageStats     storageStats
	storageNotice    string
	bringingMain     bool
}

const (
	maxDetailCacheEntries   = 64
	maxResourceCacheEntries = 24
)

func (a *app) putDetailCache(key string, p leetcode.Problem) {
	if a.detailCache == nil {
		a.detailCache = map[string]leetcode.Problem{}
	}
	if _, exists := a.detailCache[key]; !exists && len(a.detailCache) >= maxDetailCacheEntries {
		clear(a.detailCache)
	}
	a.detailCache[key] = p
}

func (a *app) purgePaidDetails() {
	for k, p := range a.detailCache {
		if p.Paid {
			delete(a.detailCache, k)
		}
	}
}

func (a *app) key() workspace.Key {
	return workspace.Key{Site: strings.ReplaceAll(string(a.site), ".", "-"), Account: a.account, Problem: a.problem.ID, Language: a.language}
}

// Serialize jobs and freeze editing when a job may replace the document.
// All model updates and jobs are initiated on the window thread.
func (a *app) job(work func() (func(), error)) {
	if a.busy || a.closed {
		return
	}
	a.busy = true
	a.restricted = leetcode.Problem{}
	a.status = "Working…"
	a.errorStatus = false
	go func() {
		done, err := work()
		a.post(func() {
			if a.closed {
				return
			}
			if a.loginWindow != nil {
				defer a.loginWindow.Update(func() {})
			}
			if a.settingsWindow != nil {
				defer a.settingsWindow.Update(func() {})
			}
			a.busy = false
			a.saving = false
			if err != nil {
				a.status = err.Error()
				a.errorStatus = true
				return
			}
			if done != nil {
				done()
			}
		})
	}()
}

func (a *app) save(next func()) {
	if a.busy || a.closed {
		return
	}
	if a.problem.ID == "" || !a.code.Buffer.Modified() {
		if next != nil {
			next()
		}
		return
	}
	key, text, store := a.key(), a.code.Buffer.String(), a.store
	version := a.code.Buffer.Version()
	a.saving = next == nil
	a.saveFailed = false
	a.job(func() (func(), error) {
		err := store.Save(key, text)
		if err != nil {
			return func() { a.saveFailed = true; a.errorStatus = true; a.status = err.Error() }, nil
		}
		return func() {
			if a.code.Buffer.Version() == version {
				a.code.Buffer.MarkSaved()
			}
			a.draftSaved = true
			a.status = "Saved locally"
			if a.code.Buffer.Modified() {
				a.status = "Snapshot saved · newer edits pending"
			}
			if next != nil {
				next()
			}
		}, err
	})
}

func (a *app) load(problem leetcode.Problem, language string) {
	a.loadWorkspace(a.site, a.account, problem, language)
}

// Commit the namespace and document together only after the draft can be read.
func (a *app) loadWorkspace(site leetcode.Site, account string, problem leetcode.Problem, language string) {
	a.loadIdentity(site, account, a.username, problem, language, "workspace")
}

func (a *app) loadIdentity(site leetcode.Site, account, username string, problem leetcode.Problem, language, screen string) {
	a.loadIdentityWithCode(site, account, username, problem, language, screen, nil)
}
func (a *app) loadIdentityWithCode(site leetcode.Site, account, username string, problem leetcode.Problem, language, screen string, code *string) {
	key := workspace.Key{Site: strings.ReplaceAll(string(site), ".", "-"), Account: account, Problem: problem.ID, Language: language}
	template := ""
	for _, s := range problem.Snippets {
		if s.Language == language {
			template = s.Code
		}
	}
	store := a.store
	a.job(func() (func(), error) {
		text, persisted, err := store.LoadDraft(key, template)
		if err != nil {
			return nil, err
		}

		return func() {
			if a.site != site {
				a.page, a.skip = leetcode.Page{}, 0
				a.listed = ""
			}
			if a.problem.ID != problem.ID || a.site != site || a.account != account {
				resKey := string(site) + ":" + account + ":" + problem.Slug
				if a.resourceCache == nil {
					a.resourceCache = map[string]map[string]*resourcePage{}
				}
				if cachedRes := a.resourceCache[resKey]; cachedRes != nil {
					a.resources = cachedRes
				} else {
					if len(a.resourceCache) >= maxResourceCacheEntries {
						clear(a.resourceCache)
					}
					a.resources = map[string]*resourcePage{}
					a.resourceCache[resKey] = a.resources
				}
				a.resetJudge()
				a.stopInlineVideo()
				a.testPanelCollapsed = true
				a.caseIndex = 0
				a.caseChipSelected = false
				a.testcases = append([]string(nil), problem.Examples...)
				a.resultTab = "testcase"
				a.resourceTab = "Description"
			}
			identityChanged := a.site != site || a.account != account
			a.site, a.account, a.username = site, account, username
			if identityChanged {
				a.refreshSolved()
			}
			a.loadingProblem = false
			a.problem, a.language = problem, language
			a.screen = screen
			if screen == "workspace" && problem.Slug != "" {
				a.lastProblemSlug = problem.Slug
				a.recordNav("workspace", problem.Slug)
				a.recordRecentProblem(problem)
				if !problem.Paid && a.win != nil {
					a.prefetchEditorial(site, problem.Slug)
				}
			}
			a.english = false
			a.saveFailed = false
			buf := editor.NewBuffer(text)
			if code != nil {
				buf.Set(*code)
			}
			buf.SetLanguage(editor.LanguageNamed(editorLanguage(language)))
			a.code.Buffer = buf
			a.version = buf.Version()
			a.draftSaved = persisted
			a.status = "Official template loaded"
			if persisted {
				a.status = "Saved locally"
			}
			a.renderDescription()
		}, err
	})
}

func (a *app) list(search string, skip int) {
	if a.catalogCancel != nil {
		a.catalogCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.catalogCancel = cancel
	a.catalogGeneration++
	generation := a.catalogGeneration
	a.catalogLoading = true
	a.catalogRetry = time.Time{}
	client := leetcode.New(a.site)
	store := a.store
	a.refreshDaily()
	go func() {
		defer cancel()
		publish := func(rows []leetcode.Problem, err error, finished bool) {
			a.post(func() {
				if a.closed || generation != a.catalogGeneration {
					return
				}
				a.catalogLoading = !finished
				if err != nil {
					a.catalogError = err.Error()
					var rate *leetcode.RateLimit
					var network net.Error
					if errors.As(err, &rate) {
						a.catalogRetry = rate.RetryAt
					} else if errors.As(err, &network) {
						a.catalogError = "Network unavailable on " + string(a.site)
						if a.catalog != nil {
							a.catalogError += " · Showing cached problems"
						}
					}
					return
				}
				a.catalogError = ""
				a.catalog = rows
				targetSkip := skip
				if a.skip > 0 {
					targetSkip = a.skip
				}
				a.filterCatalog(a.search, targetSkip)
			})
		}
		rows, fresh := readCatalogCache(store.Root, string(client.Site))
		if rows != nil {
			publish(rows, nil, fresh)
			if fresh {
				return
			}
		}
		if rows == nil {
			first, err := client.List(ctx, "", 0)
			if err == nil {
				a.post(func() {
					if !a.closed && generation == a.catalogGeneration && a.catalog == nil {
						a.page = first
					}
				})
			}
		}
		rows, err := client.Catalog(ctx)
		if err == nil && ctx.Err() == nil {
			writeCatalogCache(store.Root, string(client.Site), rows)
		}
		publish(rows, err, true)
	}()

}

func (a *app) recordRecentProblem(p leetcode.Problem) {
	if p.Slug == "" || p.ID == "" {
		return
	}
	next := []leetcode.Problem{p}
	for _, existing := range a.recentProblems {
		if existing.Slug != p.Slug {
			next = append(next, existing)
			if len(next) >= 6 {
				break
			}
		}
	}
	a.recentProblems = next
}

func (a *app) pickLanguageFor(problem leetcode.Problem) string {
	if workspace.Extension(a.language) != "" {
		for _, s := range problem.Snippets {
			if s.Language == a.language {
				return s.Language
			}
		}
	}
	for _, s := range problem.Snippets {
		if workspace.Extension(s.Language) != "" {
			return s.Language
		}
	}
	return ""
}

func (a *app) open(slug string) {
	if a.problem.Slug == slug && a.problem.ID != "" && !a.loadingProblem {
		a.navigate("workspace")
		return
	}
	a.save(func() {
		site := a.site
		cacheKey := string(site) + ":" + slug
		// 1. Check in-memory detail cache first for instant switching.
		if cached, ok := a.detailCache[cacheKey]; ok && (!cached.Paid || a.premium) {
			if lang := a.pickLanguageFor(cached); lang != "" {
				a.load(cached, lang)
				return
			}
		}
		// 2. Check known catalog metadata for instant Premium guard or instant Workspace entry.
		var meta leetcode.Problem
		hasMeta := false
		for _, p := range a.catalog {
			if p.Slug == slug {
				meta, hasMeta = p, true
				break
			}
		}
		if hasMeta && meta.Paid && (a.username == "" || (a.solved != nil && !a.premium)) {
			a.restricted = meta
			a.premiumModal = true
			return
		}
		// 3. Check on-disk public cache synchronously for public non-paid problems.
		if (!hasMeta || !meta.Paid) && a.store.Root != "" {
			if cached, ok := readDetailCache(a.store.Root, string(site), slug); ok {
				a.putDetailCache(cacheKey, cached)
				if lang := a.pickLanguageFor(cached); lang != "" {
					a.load(cached, lang)
					return
				}
			}
		}
		// 4. Immediately enter Workspace with catalog metadata so clicking feels instantaneous.
		prevScreen := a.screen
		prevProblem := a.problem
		if hasMeta && !meta.Paid {
			a.problem = meta
			a.screen = "workspace"
			a.resourceTab = "Description"
			a.loadingProblem = true
			if a.preview != nil {
				a.preview.Page().LoadHTML(render.DescriptionWithAppearance("", `<p style="color:var(--muted)">Loading problem statement…</p>`, a.appearance), "")
			}
		}
		client := leetcode.New(site)
		user := a.username
		storeRoot := a.store.Root
		a.job(func() (func(), error) {
			problem, err := client.Detail(context.Background(), slug)
			authorized := false
			if err == nil && problem.Paid && user != "" {
				cookies, viewer, e := verifiedCookiesCached(context.Background(), client, user)
				err = e
				if err == nil && viewer.Premium {
					problem, err = client.DetailForViewer(context.Background(), slug, cookies)
					authorized = err == nil && len(problem.Snippets) > 0 && problem.Content != ""
				}
			}
			if err == nil && !problem.Paid && storeRoot != "" {
				writeDetailCache(storeRoot, string(site), problem)
			}
			return func() {
				a.loadingProblem = false
				if err != nil {
					a.problem = prevProblem
					a.screen = prevScreen
					if prevProblem.ID != "" {
						a.renderDescription()
					}
					a.status = err.Error()
					a.errorStatus = true
					return
				}
				if problem.Paid && !authorized {
					a.problem = prevProblem
					a.screen = prevScreen
					if prevProblem.ID != "" {
						a.renderDescription()
					}
					a.restricted = problem
					a.premiumModal = true
					return
				}
				a.putDetailCache(cacheKey, problem)
				language := a.pickLanguageFor(problem)
				if language == "" {
					a.problem = prevProblem
					a.screen = prevScreen
					if prevProblem.ID != "" {
						a.renderDescription()
					}
					a.status = "No supported code template is available"
					a.errorStatus = true
					return
				}
				a.load(problem, language)
			}, nil
		})
	})
}

func (a *app) prefetchProblemDetail(site leetcode.Site, slug string) {
	if slug == "" || a.closed {
		return
	}
	cacheKey := string(site) + ":" + slug
	if _, ok := a.detailCache[cacheKey]; ok {
		return
	}
	storeRoot := a.store.Root
	go func() {
		if storeRoot != "" {
			if cached, ok := readDetailCache(storeRoot, string(site), slug); ok {
				a.post(func() {
					if a.closed || a.site != site {
						return
					}
					a.putDetailCache(cacheKey, cached)
				})
				return
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		problem, err := leetcode.New(site).Detail(ctx, slug)
		if err != nil || problem.Paid || problem.ID == "" {
			return
		}
		if storeRoot != "" {
			writeDetailCache(storeRoot, string(site), problem)
		}
		a.post(func() {
			if a.closed || a.site != site {
				return
			}
			a.putDetailCache(cacheKey, problem)
		})
	}()
}

func (a *app) logout() {
	a.save(func() {
		site := a.site
		invalidateSessionCache(site)
		session := mygo.NewSession(string(site))
		a.job(func() (func(), error) {
			err := session.ClearData(mygo.SiteDataAll)
			return func() {
				a.username = ""
				a.nickname = ""
				a.avatarURL = ""
				a.avatar = nil
				a.resources = nil
				a.resourceCache = nil
				a.purgePaidDetails()
				a.resetJudge()
				a.refreshSolved()
				if a.catalog != nil {
					a.filterCatalog(a.search, 0)
				}
				a.accountNotice = "Signed out. Local drafts have been retained."
				a.selectIdentity("anonymous", "")
			}, err
		})
	})
}

// resetDocument is called only after pending edits have been saved.
func (a *app) resetDocument(site leetcode.Site) {
	invalidateSessionCache(a.site)
	a.navHistory = nil
	a.navIndex = 0
	a.navReplaying = false
	a.detailCache = nil
	a.resources = nil
	a.resourceCache = nil
	a.playgrounds = nil
	a.codeTabSelected = nil
	a.slideDecks = nil
	a.linkedImages = nil
	a.mermaidShowSrc = nil
	a.recentProblems = nil
	a.lastProblemSlug = ""
	a.loadingProblem = false
	a.premiumModal = false
	a.stopInlineVideo()
	a.resetJudge()
	a.site, a.account, a.username = site, "anonymous", ""
	a.nickname, a.avatarURL, a.avatar = "", "", nil
	a.screen = "problems"
	a.premium, a.expired = false, false
	a.restricted = leetcode.Problem{}
	a.solved = nil
	a.statusFilter = ""
	a.accessFilter = ""
	a.solvedNotice = ""
	if a.catalogCancel != nil {
		a.catalogCancel()
	}
	a.catalogGeneration++
	a.catalogRetry = time.Time{}
	a.lastDailyCheck = ""
	a.catalog, a.matches = nil, nil
	a.difficulties, a.tags = nil, nil
	a.filterOpen, a.sortOpen, a.langOpen = false, false, false
	a.sitePopup, a.editorSettingsOpen = false, false
	a.quickPage, a.quickSearch = leetcode.Page{}, ""
	a.resourceTab = ""
	a.catalogScroll = ui.ScrollState{}
	a.selectedRow = -1
	a.listKeyboardNav = false
	a.hoverRowPlus1 = 0
	a.lastCatalogScrollY = 0
	a.accountNotice = ""
	a.daily, a.dailyError, a.catalogError = leetcode.Daily{}, "", ""
	a.english, a.saveFailed, a.draftSaved = false, false, false
	a.problem = leetcode.Problem{}
	if a.language == "" {
		a.language = "java"
	}
	a.page, a.skip, a.search, a.listed = leetcode.Page{}, 0, "", ""
	a.caseChipSelected = false
	a.code.Buffer = editor.NewBuffer("")
	a.version = a.code.Buffer.Version()
	a.status = "Choose a problem to start practicing"
	if a.preview != nil {
		a.preview.Page().LoadHTML(render.Description("Declic", "<p>Choose a problem to start practicing.</p>"), "")
	}
}

func (a *app) selectAccount(account string) { a.selectIdentity(account, "") }

func (a *app) selectIdentity(account, username string) {
	if account == "anonymous" {
		username = ""
	}
	if a.problem.ID == "" {
		a.account, a.username = account, username
		a.refreshSolved()
		a.status = "Browsing as guest"
		if account != "anonymous" {
			a.status = "Session verified · " + username
		}
		return
	}
	if a.problem.Paid && a.account != account {
		site := a.site
		a.resetDocument(site)
		a.selectIdentity(account, username)
		a.list("", 0)
		return
	}
	a.loadIdentity(a.site, account, username, a.problem, a.language, a.screen)
}

func (a *app) restoreOnStartup(site leetcode.Site, lastSlug string, verify func(context.Context, leetcode.Site) (leetcode.Viewer, bool, error)) {
	openLast := a.openOnLaunch == "last_workspace" && lastSlug != ""
	maybeOpenLast := func() {
		if a.closed || a.site != site || !openLast || a.screen != "problems" || a.problem.ID != "" || a.busy {
			return
		}
		for _, p := range a.catalog {
			if p.Slug == lastSlug && p.Paid && !a.premium {
				return
			}
		}
		a.open(lastSlug)
	}
	if a.noRestoreSession || verify == nil || a.post == nil {
		maybeOpenLast()
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		viewer, ok, _ := verify(ctx, site)
		a.post(func() {
			if a.closed || a.site != site {
				return
			}
			if ok && viewer.SignedIn && viewer.Username != "" && a.username == "" {
				acct := fmt.Sprintf("%x", sha256.Sum256([]byte(viewer.Username)))
				a.applyViewerProfile(viewer)
				a.selectIdentity(acct, viewer.Username)
			}
			maybeOpenLast()
		})
	}()
}

func main() {
	a := &app{site: leetcode.COM, account: "anonymous", language: "java", split: 45, showTopics: true, descriptionSize: 15, editorFontSize: 13, editorTabSize: 0, testPanelCollapsed: true, selectedRow: -1, code: &editor.State{Buffer: editor.NewBuffer(""), NoScrollPastEnd: true, SoftWrap: true}, editorSplit: 68}
	root, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	prefPath := filepath.Join(root, "ui-preferences.json")
	prefErr := a.readPreferences(prefPath)
	startupSlug := a.lastProblemSlug
	a.prefPath = prefPath
	a.prefReadErr = prefErr
	a.syncNativeTheme()
	a.lastSavedPrefs = a.preferences()
	mygo.App.WhenReady(func() {
		a.restoreDockIcon()
		a.store = workspace.Store{Root: filepath.Join(root, "Workspace")}
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Declic", TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 44, TrafficLightPosition: &mygo.Point{X: 16, Y: 14}, Width: mainWinWidth, Height: mainWinHeight, MinWidth: mainWinMinWidth, MinHeight: mainWinMinHeight, Content: ui.View(a.view)})
		a.win.Center()
		a.post = a.win.Update
		a.preview, err = a.win.NewWebView(mygo.WebViewOptions{URL: "about:blank", Page: mygo.PageOptions{Untrusted: true, Session: mygo.NewEphemeralSession()}})
		if err != nil {
			log.Fatal(err)
		}
		a.preview.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
			if e.URL != "" && e.URL != "about:blank" {
				if strings.HasPrefix(e.URL, "about:blank#") {
					return
				}
				e.PreventDefault()
				if slug, ok := render.ParseLocalProblemURL(e.URL); ok {
					a.win.Update(func() { a.open(slug) })
				} else if e.URL == render.LocalVideoSentinel {
					mygo.Shell.OpenExternal(a.problemURL())
				}
			}
		})
		a.resetDocument(a.site)
		a.lastProblemSlug = startupSlug
		if prefErr != nil {
			a.status = "Could not read settings: " + prefErr.Error()
			a.errorStatus = true
		}
		a.list("", 0)
		a.restoreOnStartup(a.site, startupSlug, verifyStartupSession)
		mygo.App.OnDidBecomeActive(func() {
			if a.loginWindow != nil && a.loginWindow.IsFocused() {
				a.bringMainBehind(a.loginWindow)
			} else if a.settingsWindow != nil && a.settingsWindow.IsFocused() {
				a.bringMainBehind(a.settingsWindow)
			}
		})
		a.win.OnClose(func(e *mygo.CloseEvent) {
			if a.busy || a.code.Buffer.Modified() {
				e.PreventDefault()
				if !a.busy {
					a.save(func() { a.win.Close() })
				}
			}
		})
		mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
			if a.busy || a.code.Buffer.Modified() {
				e.PreventDefault()
				if !a.busy {
					a.save(func() { mygo.App.Quit() })
				}
			}
		})
		done := make(chan struct{})
		a.win.OnClosed(func() {
			a.closed = true
			a.stopInlineVideo()
			a.resources = nil
			a.resetJudge()
			if a.catalogCancel != nil {
				a.catalogCancel()
			}
			close(done)
		})
		go func() {
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					a.win.Update(func() {
						if !a.closed && !a.catalogRetry.IsZero() && time.Now().After(a.catalogRetry) {
							a.list(a.search, a.skip)
						}
						if !a.closed && a.lastDailyCheck != siteToday(a.site) {
							a.refreshDaily()
						}
						if !a.closed && !a.busy && !a.saveFailed && a.code.Buffer.Modified() && time.Since(a.changed) > 800*time.Millisecond {
							a.save(nil)
						}
						a.maybePersistPreferences(time.Now())
					})
				}
			}
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	if final := a.preferences(); prefErr == nil && final != a.lastSavedPrefs {
		if err := final.write(prefPath); err != nil {
			log.Print("Saving preferences: ", err)
		}
	}
}

func (a *app) refreshDaily() {
	site, generation := a.site, a.catalogGeneration
	a.lastDailyCheck = siteToday(site)
	go func() {
		daily, err := leetcode.New(site).Daily(context.Background())
		a.post(func() {
			if a.closed || generation != a.catalogGeneration {
				return
			}
			a.dailyError = ""
			if err != nil {
				a.dailyError = err.Error()
			} else {
				a.daily = daily
				if !daily.Problem.Paid && daily.Problem.Slug != "" {
					a.prefetchProblemDetail(site, daily.Problem.Slug)
				}
			}
		})
	}()
}
