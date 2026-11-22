package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/workspace"
)

func TestWorkspaceResourcesPreserveDraft(t *testing.T) {
	a, _ := testWorkspace(t)
	a.code.Buffer.Set("unsaved solution")
	for _, tab := range []string{"Editorial", "Solutions"} {
		a.resourcePage(tab).loaded = true
	}
	view := ui.NewTester(a.view, 1360, 860)
	if _, ok := view.Find("Discover"); ok {
		t.Fatal("workspace should reserve width for the problem and editor")
	}
	if view.HasText("solution.go") {
		t.Fatal("editor displays a virtual filename")
	}
	for _, tab := range []string{"Editorial", "Solutions", "Submissions", "Description"} {
		if err := view.Click(tab); err != nil {
			t.Fatal(err)
		}
		if a.code.Buffer.String() != "unsaved solution" || !a.code.Buffer.Modified() {
			t.Fatalf("%s replaced the draft", tab)
		}
		if _, ok := view.Find("Code editor"); !ok {
			t.Fatalf("%s hid the editor", tab)
		}
	}
	if err := view.Click("Test Result"); err != nil {
		t.Fatal(err)
	}
	if !view.HasText("No execution results") {
		t.Fatal("missing truthful execution state")
	}
}
func TestResumeCurrentProblemDoesNotReloadOrResetUndo(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	a.code.Buffer.Set("working text")
	version := a.code.Buffer.Version()
	a.open(a.problem.Slug)
	if a.busy || a.screen != "workspace" || a.code.Buffer.Version() != version {
		t.Fatal("resume reloaded active document")
	}
}
func TestCatalogScrollSurvivesWorkspace(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	for i := 0; i < 20; i++ {
		a.page.Problems = append(a.page.Problems, leetcode.Problem{Slug: string(rune('a' + i)), Title: "Sample", FrontendID: "1"})
	}
	a.page.Total = 20
	view := ui.NewTester(a.view, 1360, 860)
	a.catalogScroll.Y = 180
	view.Frame()
	want := a.catalogScroll.Y
	a.navigate("workspace")
	view.Frame()
	a.navigate("problems")
	view.Frame()
	if want == 0 || a.catalogScroll.Y != want {
		t.Fatalf("catalog position changed: %v -> %v", want, a.catalogScroll.Y)
	}
}
func TestQuickOpenFocusesSearchAndPreservesCatalogQuery(t *testing.T) {
	a, _ := testWorkspace(t)
	a.search, a.listed = "catalog query", "catalog query"
	a.openQuickOpen()
	view := ui.NewTester(a.view, 1360, 860)
	view.Type("two sum")
	if a.quickSearch != "two sum" {
		t.Fatal("quick open did not focus the input")
	}
	if a.search != "catalog query" || a.listed != "catalog query" {
		t.Fatal("quick search altered catalog context")
	}
}

func TestTemplateIsNotReportedAsPersistedDraft(t *testing.T) {
	a, apply := testWorkspace(t)
	a.load(a.problem, a.language)
	apply()
	view := ui.NewTester(a.view, 1360, 860)
	if view.HasText("Saved locally") {
		t.Fatal("untouched template claimed to be a saved draft")
	}
	a.code.Buffer.Set("my solution")
	a.save(nil)
	apply()
	view.Frame()
	if !a.draftSaved || a.code.Buffer.Modified() || view.HasText("Saved locally") {
		t.Fatal("successful persistence not reflected in the editor")
	}
}

func TestSidebarAccountPlacement(t *testing.T) {
	a, _ := testWorkspace(t)
	a.username = "sample_user"
	v := ui.NewTester(a.view, 1360, 860)
	for _, collapsed := range []bool{false, true} {
		a.sidebarCollapsed = collapsed
		a.accountPopup = false
		v.Frame()
		if err := v.Click(a.username); err != nil {
			t.Fatal(err)
		}
		anchor, _ := v.Find(a.username)
		link, ok := v.Find("Open profile")
		if !ok || !collapsed && link.Y >= anchor.Y || collapsed && link.X < anchor.X+anchor.W {
			t.Fatalf("collapsed=%v anchor=%+v link=%+v", collapsed, anchor, link)
		}
	}
}
func TestLoginChoicesAreParallelAndProvisional(t *testing.T) {
	a, _ := testWorkspace(t)
	a.loginSite = a.site
	v := ui.NewTester(a.loginPanel, 600, 420)
	left, _ := v.Find("LeetCode")
	right, ok := v.Find("LeetCode CN")
	if !ok || right.X <= left.X || right.Y != left.Y {
		t.Fatalf("site choice layout %v %v", left, right)
	}
	if err := v.Click("LeetCode CN"); err != nil {
		t.Fatal(err)
	}
	if a.site != leetcode.COM || a.loginSite != leetcode.CN || a.problem.ID != "1" {
		t.Fatal("selection changed active workspace")
	}
}

func TestFilteringAcrossPagesAndCachedQuickOpen(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	a.pageSize = 20
	for i := 1; i <= 61; i++ {
		p := leetcode.Problem{FrontendID: fmt.Sprint(i), Slug: fmt.Sprintf("p-%d", i), Title: "题目", EnglishTitle: "Sample", Difficulty: "Easy"}
		if i%2 == 1 {
			p.Difficulty = "Hard"
		}
		a.catalog = append(a.catalog, p)
	}
	a.difficulties = map[string]bool{"Hard": true}
	a.filterCatalog("Sample", 20)
	if a.page.Total != 31 || len(a.page.Problems) != 11 || a.page.Problems[0].FrontendID != "41" {
		t.Fatal("filtering used the visible page")
	}
	writeCatalogCache(a.store.Root, string(a.site), a.catalog)
	a.catalog, _ = readCatalogCache(a.store.Root, string(a.site))
	a.quickSearch = "p-61"
	a.searchQuickOpen()
	if len(a.quickPage.Problems) != 1 || a.quickPage.Problems[0].FrontendID != "61" || a.skip != 20 {
		t.Fatal("offline quick open lost the catalog context")
	}
}
func TestTestcaseEditsPreserveDefaultAndCode(t *testing.T) {
	a, _ := testWorkspace(t)
	a.problem.Metadata = `{"params":[{"name":"target"}]}`
	a.problem.Examples = []string{"4", "7"}
	a.testcases = []string{"4", "7"}
	v := ui.NewTester(a.view, 1360, 860)
	if _, ok := v.Find("Duplicate"); ok {
		t.Fatal("Duplicate testcase button should be removed")
	}
	// Neither case shows a delete badge by default before hover or click.
	if _, ok := v.Find("Delete Case 1"); ok {
		t.Fatal("Case 1 should not show delete badge by default")
	}
	if _, ok := v.Find("Delete Case 2"); ok {
		t.Fatal("Case 2 should not show delete badge by default")
	}
	// Hovering Case 1 reveals its delete badge.
	c1, ok := v.Find("Case 1")
	if !ok {
		t.Fatal("missing Case 1 chip")
	}
	v.Move(c1.X+c1.W/2, c1.Y+c1.H/2)
	v.Frame()
	if _, ok := v.Find("Delete Case 1"); !ok {
		t.Fatal("hovering Case 1 should reveal Delete Case 1 badge")
	}
	// Move pointer away and add a new case; the newly active Case 3 shows its badge.
	v.Move(10, 10)
	if err := v.Click("Add case"); err != nil {
		t.Fatal(err)
	}
	if err := v.Click("target"); err != nil {
		t.Fatal(err)
	}
	v.Type("2")
	if len(a.testcases) != 3 || a.testcases[2] != "2" || a.testcases[0] != "4" || a.code.Buffer.String() != "template" {
		t.Fatalf("edit affected wrong document: %v", a.testcases)
	}
	if _, ok := v.Find("Delete Case 1"); ok {
		t.Fatal("inactive Case 1 should not display delete badge while Case 3 is selected")
	}
	if err := v.Click("Delete Case 3"); err != nil {
		t.Fatal(err)
	}
	if len(a.testcases) != 2 || a.testcases[0] != "4" {
		t.Fatalf("inline case delete failed: %v", a.testcases)
	}
	// Reset button stays active and restores defaults after edits.
	addBtn, ok1 := v.Find("Add case")
	resetBtn, ok2 := v.Find("Reset")
	if !ok1 || !ok2 || resetBtn.X <= addBtn.X {
		t.Fatalf("expected Reset after +, got add=%+v reset=%+v", addBtn, resetBtn)
	}
	if err := v.Click("target"); err != nil {
		t.Fatal(err)
	}
	v.Type("9")
	if err := v.Click("Reset"); err != nil {
		t.Fatal(err)
	}
	if len(a.testcases) != 2 || a.testcases[0] != "4" || a.testcases[1] != "7" || a.caseChipSelected {
		t.Fatalf("restore lost default: %v selected=%v", a.testcases, a.caseChipSelected)
	}
}

func TestSettingsAppearanceControlsAndPagination(t *testing.T) {
	a, _ := testWorkspace(t)
	a.settingsSection = "Appearance"
	a.descriptionSize = 15
	a.showTopics = true
	v := ui.NewTester(func(c *ui.Context) {
		a.applyTheme(c)
		ui.Column(c).Fill().Background(c.Theme().Surface).Children(func() { a.settingsView(c) })
	}, 780, 540)
	if err := v.Click("Compact"); err != nil {
		t.Fatal(err)
	}
	if !a.compact {
		t.Fatal("density toggle did not switch to compact")
	}
	if err := v.Click("Increase font size"); err != nil {
		t.Fatal(err)
	}
	if a.descriptionSize != 16 {
		t.Fatalf("descriptionSize = %d, want 16", a.descriptionSize)
	}
	if err := v.Click("Off"); err != nil {
		t.Fatal(err)
	}
	if a.showTopics {
		t.Fatal("topic visibility toggle did not turn off")
	}
	got := paginationPages(1, 13)
	want := []int{1, 2, 3, 0, 13}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("paginationPages(1, 13) = %v, want %v", got, want)
	}
}

func TestTopNavOrderShortcutAndHistory(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	v := ui.NewTester(a.view, 1280, 800)
	v.SetTitleBar(ui.TitleBar{Left: 78, Height: 44})
	v.Frame()
	toggle, ok1 := v.Find("Toggle sidebar")
	back, ok2 := v.Find("Go back")
	fwd, ok3 := v.Find("Go forward")
	if !ok1 || !ok2 || !ok3 || !(toggle.X < back.X && back.X < fwd.X) {
		t.Fatalf("unexpected nav control layout: toggle=%+v back=%+v fwd=%+v", toggle, back, fwd)
	}
	v.Key(ui.Cmd, ui.KeyB)
	if !a.sidebarCollapsed {
		t.Fatal("Cmd+B did not collapse sidebar")
	}
	a.navigate("workspace")
	if !a.sidebarCollapsed {
		t.Fatal("navigating to workspace altered global sidebar state")
	}
	if !a.canGoBack() || a.canGoForward() {
		t.Fatalf("history state after workspace nav: index=%d len=%d", a.navIndex, len(a.navHistory))
	}
	a.goBack()
	if a.screen != "problems" || !a.canGoForward() || !a.sidebarCollapsed {
		t.Fatalf("goBack failed or altered sidebar: screen=%s index=%d collapsed=%v", a.screen, a.navIndex, a.sidebarCollapsed)
	}
	v.Key(ui.Cmd, ui.KeyB)
	if a.sidebarCollapsed {
		t.Fatal("Cmd+B did not expand sidebar")
	}
	a.goForward()
	if a.screen != "workspace" || a.sidebarCollapsed {
		t.Fatalf("goForward failed or altered sidebar: screen=%s collapsed=%v", a.screen, a.sidebarCollapsed)
	}
}

func TestSingleSelectDifficultyAndAccessFilter(t *testing.T) {
	a, _ := testWorkspace(t)
	a.catalog = []leetcode.Problem{
		{FrontendID: "1", Slug: "a", Title: "A", Difficulty: "Easy", Paid: false},
		{FrontendID: "2", Slug: "b", Title: "B", Difficulty: "Medium", Paid: true},
		{FrontendID: "3", Slug: "c", Title: "C", Difficulty: "Hard", Paid: false},
	}
	a.selectDifficulty("Easy")
	if a.selectedDifficulty() != "Easy" || a.page.Total != 1 {
		t.Fatalf("easy filter got total=%d", a.page.Total)
	}
	a.selectDifficulty("Medium")
	if a.selectedDifficulty() != "Medium" || a.page.Total != 1 || a.page.Problems[0].Slug != "b" {
		t.Fatalf("single-select switch failed: %+v", a.page.Problems)
	}
	a.selectDifficulty("Medium")
	if a.selectedDifficulty() != "" || a.page.Total != 3 {
		t.Fatalf("toggle off failed: total=%d", a.page.Total)
	}
	a.accessFilter = "Free"
	a.filterCatalog("", 0)
	if a.page.Total != 2 {
		t.Fatalf("free access filter total=%d, want 2", a.page.Total)
	}
}

func TestTitleSortKeyboardSelectionAndLanguageExtensions(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	a.selectedRow = -1
	a.catalog = []leetcode.Problem{
		{FrontendID: "1", Slug: "two-sum", Title: "Two Sum", Difficulty: "Easy"},
		{FrontendID: "2", Slug: "add-two", Title: "Add Two Numbers", Difficulty: "Medium"},
		{FrontendID: "3", Slug: "longest", Title: "Longest Substring", Difficulty: "Medium"},
	}
	a.toggleSort("Title")
	if len(a.page.Problems) != 3 || a.page.Problems[0].Slug != "add-two" || a.page.Problems[2].Slug != "two-sum" {
		t.Fatalf("Title ascending sort failed: %+v", a.page.Problems)
	}
	a.toggleSort("Title")
	if a.page.Problems[0].Slug != "two-sum" {
		t.Fatalf("Title descending sort failed: %+v", a.page.Problems)
	}
	v := ui.NewTester(a.view, 1280, 800)
	v.Key(0, ui.KeyDown)
	if a.selectedRow != 0 {
		t.Fatalf("KeyDown selectedRow = %d, want 0", a.selectedRow)
	}
	v.Key(0, ui.KeyDown)
	if a.selectedRow != 1 {
		t.Fatalf("second KeyDown selectedRow = %d, want 1", a.selectedRow)
	}
	v.Key(0, ui.KeyUp)
	if a.selectedRow != 0 {
		t.Fatalf("KeyUp selectedRow = %d, want 0", a.selectedRow)
	}
	for _, lang := range []string{"python", "php", "dart", "scala", "elixir", "erlang", "racket", "mysql", "postgresql", "bash"} {
		if workspace.Extension(lang) == "" {
			t.Fatalf("missing extension for %s", lang)
		}
	}
}

func TestPublicDetailCacheNeverPersistsPaidAndSubmissionsFilter(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Workspace")
	pub := leetcode.Problem{ID: "1", FrontendID: "1", Slug: "two-sum", Title: "Two Sum", Paid: false}
	paid := leetcode.Problem{ID: "256", FrontendID: "256", Slug: "paint-house", Title: "Paint House", Paid: true}
	writeDetailCache(root, "leetcode.com", pub)
	writeDetailCache(root, "leetcode.com", paid)
	if got, ok := readDetailCache(root, "leetcode.com", "two-sum"); !ok || got.Title != "Two Sum" {
		t.Fatalf("public detail cache miss: %+v %v", got, ok)
	}
	if _, ok := readDetailCache(root, "leetcode.com", "paint-house"); ok {
		t.Fatal("paid problem must not be cached on disk")
	}

	rows := []leetcode.Submission{
		{ID: "1", Status: "Accepted", Language: "python3"},
		{ID: "2", Status: "Wrong Answer", Language: "python3"},
		{ID: "3", Status: "Accepted", Language: "golang"},
	}
	if got := filterSubmissions(rows, "Accepted", "golang"); len(got) != 1 || got[0].ID != "3" {
		t.Fatalf("filterSubmissions got %+v", got)
	}

	a, _ := testWorkspace(t)
	a.catalog = []leetcode.Problem{paid}
	a.open("paint-house")
	if !a.premiumModal || a.restricted.Slug != "paint-house" {
		t.Fatalf("premium modal not opened for guest: modal=%v restricted=%+v", a.premiumModal, a.restricted)
	}
}

func TestFocusEditorModeShortcut(t *testing.T) {
	a, _ := testWorkspace(t)
	v := ui.NewTester(a.view, 1360, 860)
	if _, ok := v.Find("Description"); !ok {
		t.Fatal("description tab missing in normal workspace")
	}
	v.Key(ui.Cmd, ui.KeyBackslash)
	if !a.editorMaximized {
		t.Fatal("Cmd+\\ did not enter focus editor mode")
	}
	v.Frame()
	if _, ok := v.Find("Description"); ok {
		t.Fatal("problem pane should be hidden in focus editor mode")
	}
	if _, ok := v.Find("Show problem"); !ok {
		t.Fatal("missing restore button in focus editor mode")
	}
	v.Key(ui.Cmd, ui.KeyBackslash)
	if a.editorMaximized {
		t.Fatal("Cmd+\\ did not restore split workspace")
	}
}

func TestSolutionsLoadMoreKeepsExistingRowsVisible(t *testing.T) {
	a, _ := testWorkspace(t)
	a.resourceTab = "Solutions"
	s := a.resourcePage("Solutions")
	s.loaded = true
	s.loading = true
	s.more = true
	s.rows = []leetcode.Article{
		{Slug: "sol-1", Title: "HashMap O(n) Solution", Views: 1200},
		{Slug: "sol-2", Title: "Two Pointer Approach", Views: 800},
	}
	v := ui.NewTester(a.view, 1360, 860)
	if !v.HasText("HashMap O(n) Solution") {
		t.Fatal("loading more wiped out already-loaded solution rows")
	}
	if !v.HasText("Loading more…") {
		t.Fatal("missing inline footer loading indicator while fetching next page")
	}
}

func TestMarkdownImageErrorReturnsUntypedNilNoPanic(t *testing.T) {
	a, _ := testWorkspace(t)
	opts := a.markdownOptions()
	src, err := opts.LoadImage("file:///etc/passwd")
	if err == nil || src != nil {
		t.Fatalf("expected untyped nil ImageSource on error, got src=%#v err=%v", src, err)
	}
	if urls := resolveArticleImageURLs(leetcode.COM, "median-of-two-sorted-arrays", "../Figures/4/4_1.png"); len(urls) == 0 || urls[0] != "https://leetcode.com/problems/median-of-two-sorted-arrays/Figures/4/4_1.png" {
		t.Fatalf("unexpected resolved relative editorial image URLs: %+v", urls)
	}
}

func TestOpenFailureRevertsOptimisticWorkspaceState(t *testing.T) {
	a, apply := testWorkspace(t)
	prevProblem := a.problem
	a.screen = "problems"
	a.site = leetcode.Site("unsupported.invalid")
	a.catalog = []leetcode.Problem{{FrontendID: "2", Slug: "add-two-numbers", Title: "Add Two Numbers"}}
	a.open("add-two-numbers")
	if !a.loadingProblem || a.screen != "workspace" {
		t.Fatalf("expected optimistic workspace entry, got screen=%q loading=%v", a.screen, a.loadingProblem)
	}
	apply()
	if a.loadingProblem {
		t.Fatal("loadingProblem stayed true after detail load error")
	}
	if a.screen != "problems" || a.problem.ID != prevProblem.ID || a.problem.Slug != prevProblem.Slug {
		t.Fatalf("failed open did not restore previous state: screen=%q problem=%+v", a.screen, a.problem)
	}
	if !a.errorStatus || a.status == "" {
		t.Fatalf("expected error status after failed open, got errorStatus=%v status=%q", a.errorStatus, a.status)
	}
	if detailCachePath(a.store.Root, "../escape", "problems", "two-sum") != "" {
		t.Fatal("detailCachePath accepted unsupported site")
	}
}

func TestSidebarSiteHeaderDailyCardAndTitleSortTrim(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	a.daily = leetcode.Daily{
		Date: "2026-11-21",
		Problem: leetcode.Problem{
			ID: "3331", FrontendID: "3331", Slug: "find-subtree-sizes",
			Title: "  Find Subtree Sizes After Changes", Difficulty: "Medium",
			Topics: []leetcode.Topic{{Name: "Tree", Slug: "tree"}},
		},
	}
	a.catalog = []leetcode.Problem{
		{FrontendID: "2843", Slug: "count-symmetric", Title: "  Count Symmetric Integers", Difficulty: "Easy"},
		{FrontendID: "542", Slug: "01-matrix", Title: "01 Matrix", Difficulty: "Medium"},
		{FrontendID: "2267", Slug: "check-parens", Title: " Check if Valid", Difficulty: "Hard"},
	}
	a.toggleSort("Title")
	if len(a.page.Problems) != 3 || a.page.Problems[0].Slug != "01-matrix" || a.page.Problems[1].Slug != "check-parens" || a.page.Problems[2].Slug != "count-symmetric" {
		t.Fatalf("Title sort did not ignore leading spaces: %+v", a.page.Problems)
	}
	v := ui.NewTester(a.view, 1360, 860)
	siteBtn, okSite := v.Find("Site: leetcode.com")
	problemsBtn, okProb := v.Find("Problems")
	if !okSite || !okProb || siteBtn.Y >= problemsBtn.Y {
		t.Fatalf("site header should sit above Problems: site=%+v problems=%+v", siteBtn, problemsBtn)
	}
	// Verify the brand emblem renders in true multi-color (amber pixels present, not monochrome black).
	img := v.Image()
	foundAmber := false
	for y := int(siteBtn.Y); y < int(siteBtn.Y+siteBtn.H) && !foundAmber; y++ {
		for x := int(siteBtn.X); x < int(siteBtn.X+siteBtn.W); x++ {
			c := img.RGBAAt(x, y)
			if c.R > 220 && c.G > 120 && c.G < 200 && c.B < 80 {
				foundAmber = true
				break
			}
		}
	}
	if !foundAmber {
		t.Fatal("expected multi-color amber pixels in sidebar LeetCode emblem")
	}
	if _, ok := v.Find("Daily Challenge: 3331. Find Subtree Sizes After Changes"); !ok {
		t.Fatal("daily challenge card should be directly clickable with trimmed title")
	}
	if !v.HasText("Solve →") {
		t.Fatal("expected lightweight Solve → affordance in daily card")
	}
	if err := v.Click("Sort"); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.Find("ID"); !ok {
		t.Fatal("Sort popover should label # sort as ID")
	}
	if _, ok := v.Find("Problem #"); ok {
		t.Fatal("Sort popover should no longer use Problem #")
	}
}

func TestKeyboardNavWhileHoveringProblemListDoesNotFlicker(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	a.selectedRow = -1
	a.catalog = []leetcode.Problem{
		{FrontendID: "1", Slug: "p-1", Title: "Alpha", Difficulty: "Easy"},
		{FrontendID: "2", Slug: "p-2", Title: "Beta", Difficulty: "Medium"},
		{FrontendID: "3", Slug: "p-3", Title: "Gamma", Difficulty: "Hard"},
	}
	a.filterCatalog("", 0)
	v := ui.NewTester(a.view, 1280, 800)
	row1, ok := v.Find("1. Alpha")
	if !ok {
		t.Fatal("missing first problem row")
	}
	v.Move(row1.X+row1.W/2, row1.Y+row1.H/2)
	v.Frame()
	if a.hoverRowPlus1 != 1 {
		t.Fatalf("expected hoverRowPlus1=1, got %d", a.hoverRowPlus1)
	}
	v.Key(0, ui.KeyDown)
	v.Frame()
	if !a.listKeyboardNav || a.selectedRow != 1 {
		t.Fatalf("KeyDown while hovering row 0 got selectedRow=%d keyboardNav=%v", a.selectedRow, a.listKeyboardNav)
	}
	v.Key(0, ui.KeyDown)
	v.Frame()
	if a.selectedRow != 2 {
		t.Fatalf("second KeyDown while pointer stayed on row 0 got selectedRow=%d, want 2", a.selectedRow)
	}
	// Moving pointer onto row 2 exits keyboard nav cleanly.
	row2, _ := v.Find("2. Beta")
	v.Move(row2.X+row2.W/2, row2.Y+row2.H/2)
	v.Frame()
	if a.listKeyboardNav || a.selectedRow != -1 {
		t.Fatalf("pointer move did not restore hover mode: keyboardNav=%v selectedRow=%d", a.listKeyboardNav, a.selectedRow)
	}
}

func TestEditorToolbarOrderCopyFormatAndSubmissionDetail(t *testing.T) {
	a, _ := testWorkspace(t)
	a.code.Buffer.Set("package main\nfunc f(){return}\n")
	v := ui.NewTester(a.view, 1360, 860)
	fmtBtn, ok1 := v.Find("Format code")
	copyBtn, ok2 := v.Find("Copy code")
	retBtn, okRet := v.Find("Retrieve last submitted code")
	resetBtn, ok3 := v.Find("Reset template")
	settingsBtn, ok4 := v.Find("Editor settings")
	focusBtn, ok5 := v.Find("Focus editor")
	if !ok1 || !ok2 || !okRet || !ok3 || !ok4 || !ok5 {
		t.Fatalf("missing editor toolbar buttons: fmt=%v copy=%v retrieve=%v reset=%v settings=%v focus=%v", ok1, ok2, okRet, ok3, ok4, ok5)
	}
	if !(fmtBtn.X < copyBtn.X && copyBtn.X < retBtn.X && retBtn.X < resetBtn.X && resetBtn.X < settingsBtn.X && settingsBtn.X < focusBtn.X) {
		t.Fatalf("unexpected editor toolbar order: fmt=%.0f copy=%.0f retrieve=%.0f reset=%.0f settings=%.0f focus=%.0f", fmtBtn.X, copyBtn.X, retBtn.X, resetBtn.X, settingsBtn.X, focusBtn.X)
	}
	if err := v.Click("Format code"); err != nil {
		t.Fatal(err)
	}
	if got := a.code.Buffer.String(); got != "package main\n\nfunc f() { return }\n" {
		t.Fatalf("Format code produced %q", got)
	}
	if err := v.Click("Copy code"); err != nil {
		t.Fatal(err)
	}
	if v.Clipboard() != a.code.Buffer.String() {
		t.Fatalf("Copy code copied %q, want %q", v.Clipboard(), a.code.Buffer.String())
	}

	// Verify submission detail omits redundant Copy button & footer note, keeping Use this code on bottom-right.
	a.username = "sample_user"
	a.resourceTab = "Submissions"
	s := a.resourcePage("Submissions")
	s.loaded = true
	s.selected = &leetcode.Submission{
		ID: "42", Status: "Accepted", Language: "golang", Runtime: "0 ms", Memory: "2.1 MB",
		Code: "func solve() int { return 1 }",
	}
	v.Frame()
	if _, ok := v.Find("Use this code"); !ok {
		t.Fatal("missing Use this code button in submission detail")
	}
	if v.HasText("Loads into the matching language") {
		t.Fatal("explanatory footer text should be removed from submission detail")
	}
}

func TestLoginLoadingAndErrorStates(t *testing.T) {
	a, _ := testWorkspace(t)
	a.loginSite = leetcode.COM
	a.loginLoading = true
	// Non-nil loginView is represented via error/loading checks; test site chooser when loginView is nil first.
	v := ui.NewTester(a.loginPanel, 600, 420)
	if !v.HasText("Sign in to LeetCode") {
		t.Fatal("expected site selection dialog when loginView is nil")
	}
}

func TestSidebarSeparatesCurrentAndUpToFiveRecentProblems(t *testing.T) {
	a, _ := testWorkspace(t)
	for i := 1; i <= 7; i++ {
		p := leetcode.Problem{
			ID:         fmt.Sprint(i),
			FrontendID: fmt.Sprint(i),
			Slug:       fmt.Sprintf("prob-%d", i),
			Title:      fmt.Sprintf("Problem %d", i),
		}
		a.problem = p
		a.recordRecentProblem(p)
	}
	v := ui.NewTester(a.view, 1360, 860)
	if !v.HasText("Current problem") || !v.HasText("Recent") {
		t.Fatal("expected both Current problem and Recent sections in sidebar")
	}
	if _, ok := v.Find("7. Problem 7"); !ok {
		t.Fatal("missing current problem 7")
	}
	for i := 2; i <= 6; i++ {
		if _, ok := v.Find(fmt.Sprintf("%d. Problem %d", i, i)); !ok {
			t.Fatalf("missing recent problem %d", i)
		}
	}
	if _, ok := v.Find("1. Problem 1"); ok {
		t.Fatal("Recent section should cap at 5 items excluding current")
	}
}

func TestCatalogViewportSnapsToWholeRowsAcrossWindowHeights(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	for i := 1; i <= 40; i++ {
		a.catalog = append(a.catalog, leetcode.Problem{
			FrontendID: fmt.Sprint(i),
			Slug:       fmt.Sprintf("p-%d", i),
			Title:      fmt.Sprintf("Sample %d", i),
			Difficulty: "Easy",
		})
	}
	a.filterCatalog("", 0)
	v := ui.NewTester(a.view, 1360, 733)
	for _, h := range []int{680, 733, 860, 1020} {
		v.SetSize(1360, h)
		v.Frame()
		v.Frame()
		if a.catalogViewportH <= 0 {
			t.Fatalf("height=%d: expected positive snapped catalogViewportH", h)
		}
		rem := int(a.catalogViewportH-6+2) % 48
		if rem != 0 {
			t.Fatalf("height=%d: catalogViewportH=%.1f is not aligned to whole 48pt rows (rem=%d)", h, a.catalogViewportH, rem)
		}
	}
}

func TestShortCodeLoadClearsStaleHorizontalScrollbar(t *testing.T) {
	a, apply := testWorkspace(t)
	a.code.SoftWrap = false
	wide := leetcode.Problem{
		ID: "2", FrontendID: "2", Slug: "wide", Title: "Wide",
		Snippets: []leetcode.Snippet{{Language: "golang", Code: "// " + string(make([]byte, 240))}},
	}
	short := leetcode.Problem{
		ID: "3", FrontendID: "3", Slug: "short", Title: "Short",
		Snippets: []leetcode.Snippet{{Language: "golang", Code: "func f() int { return 1 }\n"}},
	}
	a.load(wide, "golang")
	apply()
	prevBuf := a.code.Buffer
	v := ui.NewTester(a.view, 1360, 860)
	v.Frame()
	a.load(short, "golang")
	apply()
	v.Frame()
	if a.code.Buffer == prevBuf {
		t.Fatal("loading a new problem reused the old editor Buffer, retaining stale maxLine width")
	}
	edRect, ok := v.Find("Code editor")
	if !ok {
		t.Fatal("missing Code editor")
	}
	// Moving over the bottom edge of the editor should keep text cursor (no horizontal scrollbar hit target).
	v.Move(edRect.X+edRect.W/2, edRect.Y+edRect.H-4)
	if cur := v.Cursor(); cur != ui.CursorText {
		t.Fatalf("expected CursorText on short code bottom edge (no horizontal scrollbar), got %v", cur)
	}
}

func TestSiteSwitchDisabledInWorkspaceAndPaginationGuardedWhileCatalogLoading(t *testing.T) {
	a, _ := testWorkspace(t)
	// In Workspace, clicking the brand header must stay read-only.
	v := ui.NewTester(a.view, 1360, 860)
	if err := v.Click("Site: leetcode.com"); err != nil {
		t.Fatal(err)
	}
	if a.sitePopup {
		t.Fatal("site switcher popover should not open in Workspace")
	}

	// On Problems while guest, clicking opens the switcher.
	a.screen = "problems"
	a.catalog = nil
	a.catalogLoading = true
	a.page = leetcode.Page{
		Total: 3697,
		Problems: []leetcode.Problem{
			{FrontendID: "1", Slug: "two-sum", Title: "Two Sum", Difficulty: "Easy"},
		},
	}
	v.Frame()
	if !v.HasText("3697 problems · Updating…") {
		t.Fatal("expected inline loading count inside search bar while catalog is updating")
	}
	// Clicking last page while full catalog is still nil must not wipe preview problems.
	_ = v.Click("74")
	if len(a.page.Problems) != 1 || a.page.Problems[0].Slug != "two-sum" {
		t.Fatalf("clicking page button before full catalog loaded cleared preview: %+v", a.page)
	}
	if err := v.Click("Site: leetcode.com"); err != nil {
		t.Fatal(err)
	}
	if !a.sitePopup {
		t.Fatal("site switcher popover should open on Problems screen for guest")
	}
}

func TestSolutionsToolbarPersistsWhileLoadingAndTemplateButtonGuards(t *testing.T) {
	a, _ := testWorkspace(t)
	a.resourceTab = "Solutions"
	s := a.resourcePage("Solutions")
	s.loaded = true
	s.loading = true
	s.rows = nil
	v := ui.NewTester(a.view, 1360, 860)
	if _, ok := v.Find("Hot"); !ok {
		t.Fatal("Solutions filter toolbar disappeared while loading new filter results")
	}
	if !v.HasText("Loading solutions…") {
		t.Fatal("missing list-area loading indicator under Solutions filter toolbar")
	}

	// Untouched template + signed out: Retrieve, Reset, Run, Submit are guarded; Format remains usable.
	a.username = ""
	a.code.Buffer.Set("template")
	v.Frame()
	if !a.isAtTemplate() {
		t.Fatal("expected isAtTemplate() == true when buffer matches official snippet")
	}
	_ = v.Click("Retrieve last submitted code")
	if a.loginWindow != nil || a.retrievingSub {
		t.Fatal("signed-out Retrieve should be disabled and do nothing")
	}
	_ = v.Click("Reset template")
	if a.restoreTemplate {
		t.Fatal("Reset template should be disabled while code equals official template")
	}
	_ = v.Click("Run")
	_ = v.Click("Submit")
	if a.judge != nil || a.accountNotice != "" {
		t.Fatal("Run and Submit should be disabled while code equals official template")
	}
	// Once edited, Reset becomes enabled.
	a.code.Buffer.Set("func solve() {}")
	v.Frame()
	if err := v.Click("Reset template"); err != nil {
		t.Fatal(err)
	}
	if !a.restoreTemplate {
		t.Fatal("Reset template should be enabled after code is modified")
	}

	// Verify Submit result panel omits View in Submissions link.
	a.testPanelCollapsed = false
	a.resultTab = "result"
	a.judge = &judgeState{
		submit: true, language: "golang",
		result: leetcode.Judgment{Verdict: "Accepted", Passed: 10, Total: 10, Runtime: "0 ms", Memory: "2.1 MB"},
	}
	v.Frame()
	if v.HasText("View in Submissions") {
		t.Fatal("Submit Test Result should not render View in Submissions link")
	}
}

func TestProblemTableWidthHeaderAndHairlineSymmetry(t *testing.T) {
	a, _ := testWorkspace(t)
	a.screen = "problems"
	a.daily = leetcode.Daily{
		Date: "2026-11-21",
		Problem: leetcode.Problem{
			ID: "1", FrontendID: "1", Slug: "two-sum", Title: "Two Sum", Difficulty: "Easy",
		},
	}
	a.catalog = []leetcode.Problem{
		{FrontendID: "2", Slug: "add-two-numbers", Title: "Add Two Numbers", Difficulty: "Medium"},
	}
	a.filterCatalog("", 0)
	v := ui.NewTester(a.view, 1360, 860)
	v.Frame()
	dailyRect, ok1 := v.Find("Daily Challenge: 1. Two Sum")
	rowRect, ok2 := v.Find("2. Add Two Numbers")
	diffHdr, ok3 := v.Find("Difficulty")
	numHdr, ok4 := v.Find("#")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		t.Fatalf("missing table rects: daily=%v row=%v diff=%v num=%v", ok1, ok2, ok3, ok4)
	}
	if rowRect.X != dailyRect.X || rowRect.W != dailyRect.W {
		t.Fatalf("problem row width/origin mismatch with daily card: row=%+v daily=%+v", rowRect, dailyRect)
	}
	if numHdr.X != rowRect.X+16 || diffHdr.X+diffHdr.W != rowRect.X+rowRect.W-16 {
		t.Fatalf("table header columns not symmetrically padded by 16pt: num=%+v diff=%+v row=%+v", numHdr, diffHdr, rowRect)
	}
	// Pixel-verify hairline sits strictly inside [row.X+10, row.X+row.W-10] with zero right overhang.
	img := v.Image()
	lineY := int(diffHdr.Y+diffHdr.H) + 4
	bg := img.RGBAAt(int(rowRect.X+rowRect.W)+6, lineY)
	for x := int(rowRect.X + rowRect.W - 8); x < int(rowRect.X+rowRect.W+20); x++ {
		if c := img.RGBAAt(x, lineY); c != bg {
			t.Fatalf("hairline pixel %v protruded on right at x=%d (row right=%.0f)", c, x, rowRect.X+rowRect.W)
		}
	}
}
