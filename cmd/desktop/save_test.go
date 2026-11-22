package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
