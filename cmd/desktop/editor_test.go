package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/haeward/declic/internal/leetcode"
)

// An empty workspace must not accept edits that have no save destination.
func TestEmptyWorkspace(t *testing.T) {
	a := &app{code: &editor.State{Buffer: editor.NewBuffer("old draft")}, split: 480}
	a.resetDocument(leetcode.COM)
	a.selectIdentity("verified-account", "")
	view := ui.NewTester(a.view, 1280, 900)
	_ = view.Click("Code editor")
	view.Type("cannot save this")
	if a.code.Buffer.String() != "" || a.code.Buffer.Modified() {
		t.Fatal("empty workspace accepted an edit")
	}
	if a.site != leetcode.COM || a.account != "verified-account" || a.problem.ID != "" {
		t.Fatal("empty workspace lost its site or account")
	}
	a.resetDocument(leetcode.CN)
	view.Frame()
	if a.account != "anonymous" || a.site != leetcode.CN {
		t.Fatal("site switch retained the previous account")
	}
}

// Keep the application-specific autosave regression, without retesting editor commands.
func TestAutosavePreservesComposition(t *testing.T) {
	state := &editor.State{Buffer: editor.NewBuffer("")}
	a := &app{screen: "workspace", code: state, problem: leetcode.Problem{ID: "1"}, language: "java", split: 480}
	view := ui.NewTester(a.view, 1280, 900)
	if err := view.Click("Code editor"); err != nil {
		t.Fatal(err)
	}
	view.Compose("zhong", 5)
	a.busy, a.saving = true, true
	view.Frame()
	if active, disabled := view.TextInputActive(); !active || disabled {
		t.Fatal("autosave interrupted the input method")
	}
	view.Compose("中文", 2)
	view.Type("中文")
	if got := state.Buffer.String(); got != "中文" {
		t.Fatalf("composition was lost during autosave: %q", got)
	}
	a.saving = false
	view.Frame()
	view.Type("must not replace the document")
	if state.Buffer.String() != "中文" {
		t.Fatal("document transition accepted an edit")
	}
}
