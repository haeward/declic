package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/haeward/declic/internal/leetcode"
)

func TestCatalogCacheUnifiedPathAndLegacyFallback(t *testing.T) {
	base := t.TempDir()
	wsRoot := filepath.Join(base, "Workspace")
	rows := []leetcode.Problem{{FrontendID: "1", Slug: "two-sum", Title: "Two Sum"}}

	writeCatalogCache(wsRoot, string(leetcode.COM), rows)
	canonical := filepath.Join(base, "cache", "catalog", "leetcode.com.json")
	if _, err := os.Stat(canonical); err != nil {
		t.Fatalf("expected catalog cache at %s: %v", canonical, err)
	}
	if _, err := os.Stat(filepath.Join(wsRoot, "cache", "leetcode.com.json")); !os.IsNotExist(err) {
		t.Fatal("new catalog cache must not be written inside Workspace/")
	}
	got, fresh := readCatalogCache(wsRoot, string(leetcode.COM))
	if !fresh || len(got) != 1 || got[0].Slug != "two-sum" {
		t.Fatalf("unexpected canonical catalog read: fresh=%v rows=%+v", fresh, got)
	}

	// Legacy read-only fallback for caches previously stored in Workspace/cache/.
	legacyPath := filepath.Join(wsRoot, "cache", "leetcode.cn.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(`[{"frontendQuestionId":"1","titleSlug":"two-sum","title":"Two Sum","titleCn":"两数之和"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	legacyRows, legacyFresh := readCatalogCache(wsRoot, string(leetcode.CN))
	if !legacyFresh || len(legacyRows) != 1 || legacyRows[0].Title != "两数之和" {
		t.Fatalf("legacy fallback failed: fresh=%v rows=%+v", legacyFresh, legacyRows)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy fallback must not delete existing file: %v", err)
	}
}

func TestDetailCachePurgesPaidAndCapsSize(t *testing.T) {
	a, _ := testWorkspace(t)
	a.putDetailCache("leetcode.com:two-sum", leetcode.Problem{ID: "1", Slug: "two-sum", Paid: false})
	a.putDetailCache("leetcode.com:paint-house", leetcode.Problem{ID: "256", Slug: "paint-house", Paid: true})
	a.expireSession()
	if _, ok := a.detailCache["leetcode.com:paint-house"]; ok {
		t.Fatal("paid problem remained in memory after session expiry")
	}
	if _, ok := a.detailCache["leetcode.com:two-sum"]; !ok {
		t.Fatal("public problem was unexpectedly evicted on session expiry")
	}

	for i := 0; i < maxDetailCacheEntries+10; i++ {
		slug := fmt.Sprintf("p-%d", i)
		a.putDetailCache("leetcode.com:"+slug, leetcode.Problem{ID: fmt.Sprint(i), Slug: slug})
	}
	if len(a.detailCache) > maxDetailCacheEntries {
		t.Fatalf("detailCache exceeded cap: %d > %d", len(a.detailCache), maxDetailCacheEntries)
	}
}

func TestResetDocumentClearsCrossSiteHistoryAndCaches(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	a.navigate("workspace")
	a.putDetailCache("leetcode.com:sample", a.problem)
	a.playgrounds = map[string]*playgroundState{"k": {}}
	a.slideDecks = map[string]*slideDeckState{"k": {}}
	a.linkedImages = map[string]*linkedImageState{"k": {}}
	a.mermaidShowSrc = map[string]bool{"k": true}

	if !a.canGoBack() {
		t.Fatal("expected back history before site switch")
	}
	a.resetDocument(leetcode.CN)
	if a.canGoBack() || a.canGoForward() {
		t.Fatalf("stale navigation history survived site switch: index=%d len=%d", a.navIndex, len(a.navHistory))
	}
	if len(a.detailCache) != 0 || len(a.playgrounds) != 0 || len(a.slideDecks) != 0 || len(a.linkedImages) != 0 || len(a.mermaidShowSrc) != 0 {
		t.Fatal("transient caches were not cleared on site switch")
	}
}

func TestDebouncedPreferencesPersistence(t *testing.T) {
	a, apply := testWorkspace(t)
	prefPath := filepath.Join(t.TempDir(), "nested", "ui-preferences.json")
	a.prefPath = prefPath
	a.lastSavedPrefs = a.preferences()

	a.appearance = "Dark"
	t0 := time.Now()
	a.maybePersistPreferences(t0)
	if !a.prefDirty || a.prefSaving {
		t.Fatalf("expected dirty without immediate write: dirty=%v saving=%v", a.prefDirty, a.prefSaving)
	}
	a.maybePersistPreferences(t0.Add(700 * time.Millisecond))
	if !a.prefSaving {
		t.Fatal("expected debounced preference write to start")
	}
	apply()
	if a.prefDirty || a.prefSaving || a.lastSavedPrefs.Theme != "Dark" {
		t.Fatalf("unexpected state after preference write: dirty=%v saving=%v theme=%q", a.prefDirty, a.prefSaving, a.lastSavedPrefs.Theme)
	}

	b, _ := testWorkspace(t)
	if err := b.readPreferences(prefPath); err != nil {
		t.Fatalf("failed reading persisted preferences: %v", err)
	}
	if b.appearance != "Dark" {
		t.Fatalf("expected Theme=Dark, got %q", b.appearance)
	}

	// Corrupted preferences read error must prevent overwriting on disk.
	a.prefReadErr = fmt.Errorf("corrupt")
	a.appearance = "Light"
	a.maybePersistPreferences(t0.Add(2 * time.Second))
	if a.prefSaving {
		t.Fatal("must not overwrite preferences after read error")
	}
}

func TestCrossPlatformShortcuts(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	a.testPanelCollapsed = true
	v := ui.NewTester(a.view, 1280, 800)
	v.Frame()

	v.Key(ui.Ctrl, ui.KeyP)
	if !a.showQuickOpen {
		t.Fatal("Ctrl+P did not open Quick Open")
	}
	a.showQuickOpen = false

	a.navigate("workspace")
	v.Frame()
	v.Key(ui.Ctrl, ui.KeyJ)
	if a.testPanelCollapsed {
		t.Fatal("Ctrl+J did not expand test panel")
	}

	v.Key(ui.Alt, ui.KeyLeft)
	if a.screen != "problems" {
		t.Fatalf("Alt+Left did not navigate back: screen=%s", a.screen)
	}
	v.Key(ui.Alt, ui.KeyRight)
	if a.screen != "workspace" {
		t.Fatalf("Alt+Right did not navigate forward: screen=%s", a.screen)
	}
}

func TestStorageScanAndClearCachesPreservesDrafts(t *testing.T) {
	base := t.TempDir()
	wsRoot := filepath.Join(base, "Workspace")
	draftPath := filepath.Join(wsRoot, "leetcode-com", "anonymous", "1", "golang", "solution.go")
	if err := os.MkdirAll(filepath.Dir(draftPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draftPath, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	prefPath := filepath.Join(base, "ui-preferences.json")
	if err := os.WriteFile(prefPath, []byte(`{"Version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	writeCatalogCache(wsRoot, string(leetcode.COM), []leetcode.Problem{{FrontendID: "1", Slug: "two-sum", Title: "Two Sum"}})
	writeDetailCache(wsRoot, string(leetcode.COM), leetcode.Problem{ID: "1", FrontendID: "1", Slug: "two-sum", Title: "Two Sum"})

	before := scanStorageStats(wsRoot)
	if before.draftCount != 1 || before.draftBytes != int64(len("package main\n")) || before.cacheBytes <= 0 {
		t.Fatalf("unexpected storage stats before clear: %+v", before)
	}
	if err := clearPublicCaches(wsRoot); err != nil {
		t.Fatalf("clearPublicCaches failed: %v", err)
	}
	after := scanStorageStats(wsRoot)
	if after.cacheBytes != 0 || after.draftCount != 1 {
		t.Fatalf("expected caches cleared and drafts preserved: %+v", after)
	}
	if _, err := os.Stat(draftPath); err != nil {
		t.Fatalf("draft file was touched by cache clear: %v", err)
	}
	if _, err := os.Stat(prefPath); err != nil {
		t.Fatalf("preferences file was touched by cache clear: %v", err)
	}
}

func TestIndentAutoAndStartupPrefs(t *testing.T) {
	a, _ := testWorkspace(t)
	a.editorTabSize = 0
	a.language = "typescript"
	v := ui.NewTester(a.view, 1280, 800)
	v.Frame()
	if a.code.TabSize != 0 || a.code.Indent != editor.IndentAuto {
		t.Fatalf("Auto tab width did not configure IndentAuto: tab=%d indent=%v", a.code.TabSize, a.code.Indent)
	}
	if effectiveTabSize("typescript", 0) != 2 || effectiveTabSize("python3", 0) != 4 || effectiveTabSize("typescript", 4) != 4 {
		t.Fatal("unexpected effectiveTabSize language defaults")
	}

	// Verify startup preference round-trip.
	prefPath := filepath.Join(t.TempDir(), "ui-preferences.json")
	a.openOnLaunch = "last_workspace"
	a.lastProblemSlug = "two-sum"
	a.noRestoreSession = true
	if err := a.preferences().write(prefPath); err != nil {
		t.Fatal(err)
	}
	b, _ := testWorkspace(t)
	if err := b.readPreferences(prefPath); err != nil {
		t.Fatal(err)
	}
	if b.openOnLaunch != "last_workspace" || b.lastProblemSlug != "two-sum" || !b.noRestoreSession || b.editorTabSize != 0 {
		t.Fatalf("startup/editor prefs did not round-trip: open=%q slug=%q noRestore=%v tab=%d", b.openOnLaunch, b.lastProblemSlug, b.noRestoreSession, b.editorTabSize)
	}
}

func TestStartupSessionAndWorkspaceRestore(t *testing.T) {
	a, apply := testWorkspace(t)
	a.screen = "problems"
	a.problem = leetcode.Problem{}
	a.openOnLaunch = "last_workspace"
	cached := leetcode.Problem{
		ID:         "1",
		FrontendID: "1",
		Slug:       "two-sum",
		Title:      "Two Sum",
		Paid:       true,
		Snippets:   []leetcode.Snippet{{Language: "golang", Code: "func twoSum() {}"}},
	}
	a.catalog = []leetcode.Problem{cached}
	a.putDetailCache("leetcode.com:two-sum", cached)

	a.restoreOnStartup(leetcode.COM, "two-sum", func(ctx context.Context, site leetcode.Site) (leetcode.Viewer, bool, error) {
		return leetcode.Viewer{SignedIn: true, Premium: true, Username: "alice", RealName: "Alice"}, true, nil
	})
	// First posted callback applies verified viewer and triggers opening last problem.
	apply()
	// Second posted callback finishes loading draft for the verified account namespace.
	apply()

	wantAcct := fmt.Sprintf("%x", sha256.Sum256([]byte("alice")))
	if a.username != "alice" || !a.premium || a.account != wantAcct || a.screen != "workspace" || a.problem.Slug != "two-sum" {
		t.Fatalf("startup restore failed: user=%q premium=%v acct=%q screen=%q slug=%q", a.username, a.premium, a.account, a.screen, a.problem.Slug)
	}

	// Disabled session restore must not invoke verifier.
	b, _ := testWorkspace(t)
	b.screen = "problems"
	b.problem = leetcode.Problem{}
	b.noRestoreSession = true
	called := false
	b.restoreOnStartup(leetcode.COM, "two-sum", func(ctx context.Context, site leetcode.Site) (leetcode.Viewer, bool, error) {
		called = true
		return leetcode.Viewer{SignedIn: true, Username: "bob"}, true, nil
	})
	if called || b.username != "" {
		t.Fatalf("restoreOnStartup verified session when disabled: called=%v user=%q", called, b.username)
	}
}

func TestWindowCenteringGeometry(t *testing.T) {
	// Standard centering over main window.
	x, y, ok := centeredPosition(
		mygo.Rectangle{X: 100, Y: 50, Width: mainWinWidth, Height: mainWinHeight},
		mygo.Rectangle{Width: settingsWinWidth, Height: settingsWinHeight},
	)
	if !ok || x != 390 || y != 210 {
		t.Fatalf("settings center over main = (%d,%d,%v), want (390,210,true)", x, y, ok)
	}

	// Secondary display with negative origin must preserve negative coordinates.
	x, y, ok = centeredPosition(
		mygo.Rectangle{X: -1500, Y: -200, Width: 1360, Height: 860},
		mygo.Rectangle{Width: loginPickerWidth, Height: loginPickerHeight},
	)
	if !ok || x != -1060 || y != 76 {
		t.Fatalf("multi-monitor center = (%d,%d,%v), want (-1060,76,true)", x, y, ok)
	}

	// Child larger than parent must fall back to screen centering.
	if _, _, ok := centeredPosition(
		mygo.Rectangle{X: 0, Y: 0, Width: settingsWinWidth, Height: settingsWinHeight},
		mygo.Rectangle{Width: loginWebWidth, Height: loginWebHeight},
	); ok {
		t.Fatal("child larger than parent should fall back to screen center")
	}
}
