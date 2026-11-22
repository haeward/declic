package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/workspace"
)

func testWorkspace(t *testing.T) (*app, func()) {
	t.Helper()
	posted := make(chan func(), 1)
	a := &app{post: func(f func()) { posted <- f }, site: leetcode.COM, account: "anonymous", screen: "workspace", language: "golang", split: 48,
		code: &editor.State{Buffer: editor.NewBuffer("template")}, store: workspace.Store{Root: t.TempDir()},
		problem: leetcode.Problem{ID: "1", FrontendID: "1", Slug: "sample", Title: "Sample", Snippets: []leetcode.Snippet{{Language: "golang", Code: "template"}}}}
	apply := func() {
		t.Helper()
		select {
		case fn := <-posted:
			fn()
		case <-time.After(5 * time.Second):
			t.Fatal("job did not finish")
		}
	}
	return a, apply
}

func TestBackToProblemsWaitsForSaveAndRetainsCatalog(t *testing.T) {
	a, apply := testWorkspace(t)
	a.search, a.listed, a.skip = "sample", "sample", 20
	a.code.Buffer.Set("my solution")
	view := ui.NewTester(a.view, 1360, 860)
	if err := view.Click("Problems"); err != nil {
		t.Fatal(err)
	}
	if a.screen != "workspace" {
		t.Fatal("left before saving")
	}
	apply()
	if a.screen != "problems" || a.skip != 20 || a.search != "sample" {
		t.Fatal("lost catalog context")
	}
	text, err := a.store.Load(a.key(), "")
	if err != nil || text != "my solution" {
		t.Fatalf("draft=%q err=%v", text, err)
	}
}

func TestIdentityLoadFailurePreservesActiveDocument(t *testing.T) {
	a, apply := testWorkspace(t)
	a.username = "old-user"
	blocked := filepath.Join(a.store.Root, "blocked")
	if err := os.WriteFile(blocked, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	a.store.Root = blocked
	a.selectIdentity("new-account", "new-user")
	apply()
	if a.account != "anonymous" || a.username != "old-user" || a.code.Buffer.String() != "template" {
		t.Fatal("failed draft load replaced active identity or text")
	}
}

func TestRestoreTemplateNeedsConfirmation(t *testing.T) {
	a, _ := testWorkspace(t)
	a.code.Buffer.Reset("previously saved solution")
	a.code.Buffer.Set("keep my solution")
	a.restoreTemplate = true
	view := ui.NewTester(a.view, 1360, 860)
	if err := view.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	if a.code.Buffer.String() != "keep my solution" {
		t.Fatal("cancel replaced the code")
	}
	a.restoreTemplate = true
	view.Frame()
	if err := view.Click("Restore template"); err != nil {
		t.Fatal(err)
	}
	if a.code.Buffer.String() != "template" || !a.code.Buffer.Modified() {
		t.Fatal("confirmed template is not pending persistence")
	}
}

func TestNarrowWorkspaceSwitchesPanels(t *testing.T) {
	a, _ := testWorkspace(t)
	view := ui.NewTester(a.view, 960, 680)
	if err := view.Click("Description"); err != nil {
		t.Fatal(err)
	}
	if _, ok := view.Find("Code editor"); ok {
		t.Fatal("narrow description still shows editor")
	}
	if err := view.Click("Editor"); err != nil {
		t.Fatal(err)
	}
	if _, ok := view.Find("Code editor"); !ok {
		t.Fatal("editor did not return")
	}
}

func TestVerifiedIdentityCommitsWithItsDraftAndKeepsCatalog(t *testing.T) {
	a, apply := testWorkspace(t)
	a.screen = "problems"
	a.search, a.listed, a.skip = "sample", "sample", 20
	key := a.key()
	key.Account = "verified-account"
	if err := a.store.Save(key, ""); err != nil {
		t.Fatal(err)
	}
	a.selectIdentity("verified-account", "sample_user")
	if a.account != "anonymous" || a.username != "" {
		t.Fatal("identity changed before its draft loaded")
	}
	apply()
	if a.account != "verified-account" || a.username != "sample_user" || a.code.Buffer.String() != "" || a.code.Buffer.Modified() {
		t.Fatal("identity and empty account draft were not committed together")
	}
	if a.screen != "problems" || a.skip != 20 || a.search != "sample" {
		t.Fatal("identity verification lost catalog state")
	}
}

func TestEditorSettingsPreserveWorkspace(t *testing.T) {
	a, _ := testWorkspace(t)
	a.editorFontSize = 13
	a.editorTabSize = 4
	a.code.SoftWrap = true
	a.code.Buffer.Set("pending work")
	view := ui.NewTester(a.view, 1360, 860)
	if err := view.Click("Editor settings"); err != nil {
		t.Fatal(err)
	}
	if !a.editorSettingsOpen || a.settingsWindow != nil || a.screen != "workspace" || a.code.Buffer.String() != "pending work" {
		t.Fatal("quick editor settings did not open inline or lost workspace context")
	}
	if err := view.Click("Increase quick font size"); err != nil {
		t.Fatal(err)
	}
	if err := view.Click("2 spaces"); err != nil {
		t.Fatal(err)
	}
	if err := view.Click("Word wrap Off"); err != nil {
		t.Fatal(err)
	}
	if a.editorFontSize != 14 || a.editorTabSize != 2 || a.code.SoftWrap {
		t.Fatalf("quick settings did not update preferences: font=%d tab=%d wrap=%v", a.editorFontSize, a.editorTabSize, a.code.SoftWrap)
	}
}
