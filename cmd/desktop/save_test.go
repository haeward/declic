package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/workspace"
)

func TestDelayedSaveCompletion(t *testing.T) {
	posted := make(chan func(), 1)
	apply := func() {
		t.Helper()
		select {
		case fn := <-posted:
			fn()
		case <-time.After(5 * time.Second):
			t.Fatal("save did not publish its result")
		}
	}
	root := t.TempDir()
	a := &app{
		post: func(fn func()) { posted <- fn }, store: workspace.Store{Root: root},
		site: leetcode.CN, account: "anonymous", problem: leetcode.Problem{ID: "1"}, language: "java",
		code: &editor.State{Buffer: editor.NewBuffer("")},
	}
	buffer := a.code.Buffer
	buffer.Set("older")
	a.save(nil)
	buffer.Set("newer")
	switched := false
	a.save(func() { switched = true })
	apply()
	if switched || !buffer.Modified() || buffer.String() != "newer" {
		t.Fatal("old save completion replaced or cleared the new edit")
	}
	a.save(func() { switched = true })
	apply()
	got, persisted, err := a.store.LoadDraft(a.key(), "template")
	if !switched || buffer.Modified() || err != nil || !persisted || got != "newer" {
		t.Fatalf("latest save did not finish before transition: text=%q persisted=%v err=%v", got, persisted, err)
	}

	// A failed write must prevent a transition and retain the dirty buffer.
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	a.store.Root = blocked
	buffer.Set("unsaved")
	switched = false
	a.save(func() { switched = true })
	apply()
	if switched || !buffer.Modified() || buffer.String() != "unsaved" {
		t.Fatal("failed save did not preserve the active document")
	}

	a.job(func() (func(), error) { return func() { switched = true }, nil })
	a.closed = true
	apply()
	if switched {
		t.Fatal("a late result updated a closed window")
	}
}

func TestBeforeQuitSessionEndingFlushesSynchronously(t *testing.T) {
	root := t.TempDir()
	posted := make(chan func(), 4)
	prefPath := filepath.Join(root, "ui-preferences.json")
	a := &app{
		post: func(fn func()) { posted <- fn }, store: workspace.Store{Root: root},
		site: leetcode.COM, account: "anonymous", problem: leetcode.Problem{ID: "42"}, language: "golang",
		code: &editor.State{Buffer: editor.NewBuffer("")}, prefPath: prefPath, appearance: "System",
	}
	a.lastSavedPrefs = a.preferences()

	// Normal quit prevents default and saves asynchronously before invoking quit.
	a.code.Buffer.Set("package main // normal")
	quitCalled := false
	normalQuit := &mygo.QuitEvent{}
	a.handleBeforeQuit(normalQuit, func() { quitCalled = true })
	if !normalQuit.DefaultPrevented() {
		t.Fatal("normal quit with dirty buffer must prevent default")
	}
	select {
	case fn := <-posted:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("normal quit save did not complete")
	}
	if !quitCalled || a.code.Buffer.Modified() {
		t.Fatalf("normal quit did not finish save: quitCalled=%v modified=%v", quitCalled, a.code.Buffer.Modified())
	}

	// OS session ending flushes synchronously when idle without preventing default or calling quit,
	// and also persists pending preferences before returning.
	a.code.Buffer.Set("package main // session ending")
	a.appearance = "Dark"
	quitCalled = false
	endEvt := &mygo.QuitEvent{SessionEnding: true}
	a.handleBeforeQuit(endEvt, func() { quitCalled = true })
	if endEvt.DefaultPrevented() || quitCalled || a.code.Buffer.Modified() || !a.sessionEnding {
		t.Fatalf("session ending state mismatch: prevented=%v quit=%v modified=%v sessionEnding=%v",
			endEvt.DefaultPrevented(), quitCalled, a.code.Buffer.Modified(), a.sessionEnding)
	}
	got, persisted, err := a.store.LoadDraft(a.key(), "template")
	if err != nil || !persisted || got != "package main // session ending" {
		t.Fatalf("session ending draft not persisted synchronously: text=%q persisted=%v err=%v", got, persisted, err)
	}
	checkPrefs := &app{code: &editor.State{Buffer: editor.NewBuffer("")}}
	if err := checkPrefs.readPreferences(prefPath); err != nil || checkPrefs.appearance != "Dark" {
		t.Fatalf("session ending preferences not persisted synchronously: appearance=%q err=%v", checkPrefs.appearance, err)
	}

	// Subsequent OnClose during session ending must not prevent default or launch an async save.
	a.code.Buffer.Set("package main // after flush")
	closeEvt := &mygo.CloseEvent{}
	closeCalled := false
	a.handleWindowClose(closeEvt, func() { closeCalled = true })
	if closeEvt.DefaultPrevented() || closeCalled || a.busy {
		t.Fatalf("OnClose during session ending should be a no-op: prevented=%v close=%v busy=%v",
			closeEvt.DefaultPrevented(), closeCalled, a.busy)
	}

	// P1 regression: an in-flight autosave of A whose completion is still queued must finish
	// before SessionEnding flushes newer edit B, leaving B on disk.
	a.sessionEnding = false
	a.code.Buffer.Set("package main // A")
	a.save(nil)
	a.code.Buffer.Set("package main // B")
	a.appearance = "Light"
	inFlightEvt := &mygo.QuitEvent{SessionEnding: true}
	a.handleBeforeQuit(inFlightEvt, func() { quitCalled = true })
	if inFlightEvt.DefaultPrevented() || quitCalled || a.code.Buffer.Modified() {
		t.Fatalf("in-flight autosave session ending mismatch: prevented=%v quit=%v modified=%v",
			inFlightEvt.DefaultPrevented(), quitCalled, a.code.Buffer.Modified())
	}
	got, persisted, err = a.store.LoadDraft(a.key(), "template")
	if err != nil || !persisted || got != "package main // B" {
		t.Fatalf("session ending dropped edit B after in-flight autosave A: text=%q persisted=%v err=%v", got, persisted, err)
	}
	// Drain queued completion of A so goroutine resources settle cleanly.
	select {
	case fn := <-posted:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("autosave A completion never posted")
	}

	// Session ending save failure leaves buffer modified and does not prevent default.
	a.sessionEnding = false
	blocked := filepath.Join(root, "blocked-root")
	if err := os.WriteFile(blocked, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	a.store.Root = blocked
	a.code.Buffer.Set("package main // cannot write")
	failEvt := &mygo.QuitEvent{SessionEnding: true}
	a.handleBeforeQuit(failEvt, func() { quitCalled = true })
	if failEvt.DefaultPrevented() || quitCalled || !a.code.Buffer.Modified() {
		t.Fatalf("failed session ending write should keep buffer modified: prevented=%v quit=%v modified=%v",
			failEvt.DefaultPrevented(), quitCalled, a.code.Buffer.Modified())
	}
}

func TestDraftSavePreservesMixedLineEndings(t *testing.T) {
	root := t.TempDir()
	posted := make(chan func(), 1)
	a := &app{
		post: func(fn func()) { posted <- fn }, store: workspace.Store{Root: root},
		site: leetcode.COM, account: "anonymous", problem: leetcode.Problem{ID: "7"}, language: "python3",
		code: &editor.State{Buffer: editor.NewBuffer("line1\r\nline2\rline3\n")},
	}
	a.code.Buffer.MarkSaved()
	// Append an edit at the end without disturbing earlier lines' CRLF / lone-CR endings.
	end := a.code.Buffer.Len()
	a.code.Buffer.Edit(editor.Edit{Start: end, End: end, Text: "line4"})
	a.save(nil)
	select {
	case fn := <-posted:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("save did not complete")
	}
	got, persisted, err := a.store.LoadDraft(a.key(), "")
	want := "line1\r\nline2\rline3\nline4"
	if err != nil || !persisted || got != want {
		t.Fatalf("draft save altered original line endings: got=%q want=%q persisted=%v err=%v", got, want, persisted, err)
	}
}
