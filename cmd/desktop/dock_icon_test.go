package main

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDockIconAssetsValid(t *testing.T) {
	seenIDs := make(map[string]bool, len(dockIcons))
	seenLabels := make(map[string]bool, len(dockIcons))
	if dockIcons[0].id != "" {
		t.Fatalf("first dock icon must be default with empty id, got %q", dockIcons[0].id)
	}
	previews := dockIconPreviews(false)
	darkPreviews := dockIconPreviews(true)
	if runtime.GOOS == "darwin" {
		if darkPreviews[0] == nil || darkPreviews[0] == previews[0] || bytes.Equal(ticketDarkPreview, ticketPreview) {
			t.Fatal("expected a distinct dark ticket preview")
		}
		if w, h := darkPreviews[0].Size(); w != 128 || h != 128 {
			t.Fatalf("dark ticket preview is %dx%d, want 128x128", w, h)
		}
		darkImg, err := png.Decode(bytes.NewReader(ticketDarkPreview))
		if err != nil {
			t.Fatalf("dark ticket preview: %v", err)
		}
		db := darkImg.Bounds()
		if _, _, _, alpha := darkImg.At(db.Min.X, db.Min.Y).RGBA(); alpha != 0 {
			t.Fatal("dark ticket preview: expected transparent outer margin")
		}
	} else if len(ticketDarkPreview) != 0 {
		t.Fatal("dark ticket preview should only be embedded on darwin")
	}
	for i, icon := range dockIcons {
		if seenIDs[icon.id] {
			t.Fatalf("duplicate dock icon id %q", icon.id)
		}
		seenIDs[icon.id] = true
		if icon.label == "" || seenLabels[icon.label] {
			t.Fatalf("invalid or duplicate dock icon label %q", icon.label)
		}
		seenLabels[icon.label] = true

		if runtime.GOOS != "darwin" {
			if len(icon.png) != 0 {
				t.Fatalf("%s: non-darwin build should not embed Dock icon PNG", icon.label)
			}
			continue
		}
		if len(icon.png) == 0 {
			t.Fatalf("%s: missing embedded PNG", icon.label)
		}
		img, err := png.Decode(bytes.NewReader(icon.png))
		if err != nil {
			t.Fatalf("%s: %v", icon.label, err)
		}
		b := img.Bounds()
		if b.Dx() != b.Dy() || b.Dx() < 128 {
			t.Fatalf("%s: expected square PNG >= 128px, got %dx%d", icon.label, b.Dx(), b.Dy())
		}
		if _, _, _, alpha := img.At(b.Min.X, b.Min.Y).RGBA(); alpha != 0 {
			t.Fatalf("%s: expected transparent outer margin", icon.label)
		}
		if previews[i] == nil {
			t.Fatalf("%s: preview bitmap failed to decode", icon.label)
		}
		if icon.id != "" && previews[i] != darkPreviews[i] {
			t.Fatalf("%s: the preview should match the fixed-color Dock icon in either theme", icon.label)
		}
	}
}

func TestDockIconPickerAndPreferences(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Dock icon picker assets are only embedded on darwin")
	}
	a, _ := testWorkspace(t)
	var applied []byte
	var calls int
	var applyErr error
	view := ui.NewTester(func(c *ui.Context) {
		a.dockIconPicker(c, func(data []byte) error {
			calls++
			applied = data
			return applyErr
		})
	}, 352, 160)

	// Clicking the already-active Default icon should be a no-op.
	if err := view.Click("Default Dock icon"); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("redundant SetIcon call on already-selected default icon: %d", calls)
	}

	path := filepath.Join(t.TempDir(), "ui-preferences.json")
	for _, icon := range dockIcons[1:] {
		prevCalls := calls
		if err := view.Click(icon.label + " Dock icon"); err != nil {
			t.Fatal(err)
		}
		if calls != prevCalls+1 || a.dockIconID != icon.id || a.dockIconPref != icon.id || !bytes.Equal(applied, icon.png) {
			t.Fatalf("wrong icon applied for %s", icon.label)
		}
		// Clicking the same active icon again should not call SetIcon again.
		if err := view.Click(icon.label + " Dock icon"); err != nil {
			t.Fatal(err)
		}
		if calls != prevCalls+1 {
			t.Fatalf("redundant SetIcon call when re-clicking %s", icon.label)
		}
		if err := a.preferences().write(path); err != nil {
			t.Fatal(err)
		}
		restarted, _ := testWorkspace(t)
		if err := restarted.readPreferences(path); err != nil || restarted.dockIconPref != icon.id || restarted.dockIconID != "" {
			t.Fatalf("restoring %s: pref=%q active=%q err=%v", icon.label, restarted.dockIconPref, restarted.dockIconID, err)
		}
	}

	// Arrow navigation inside RadioGroup should move focus and apply the adjacent icon.
	view.Key(0, ui.KeyLeft)
	if a.dockIconID != "pruners" || a.dockIconPref != "pruners" || !bytes.Equal(applied, prunersIcon) {
		t.Fatalf("KeyLeft did not switch to pruners: id=%q pref=%q", a.dockIconID, a.dockIconPref)
	}
	view.Key(0, ui.KeyRight)
	if a.dockIconID != "nesting" || a.dockIconPref != "nesting" || !bytes.Equal(applied, nestingIcon) {
		t.Fatalf("KeyRight did not switch back to nesting: id=%q pref=%q", a.dockIconID, a.dockIconPref)
	}

	applyErr = errors.New("unavailable")
	if err := view.Click("Default Dock icon"); err != nil {
		t.Fatal(err)
	}
	if a.dockIconID != "nesting" || a.dockIconPref != "nesting" || a.dockIconError == "" {
		t.Fatal("failed switch lost the current choice or error message")
	}
	applyErr = nil
	if err := view.Click("Default Dock icon"); err != nil {
		t.Fatal(err)
	}
	if applied != nil || a.dockIconID != "" || a.dockIconPref != "" || a.dockIconError != "" {
		t.Fatal("default did not restore the native bundle icon")
	}
}

func TestRestoreDockIcon(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Dock icon assets are only embedded on darwin")
	}

	// Default icon should not invoke SetIcon on startup.
	a, _ := testWorkspace(t)
	called := false
	a.restoreDockIconWith(func([]byte) error {
		called = true
		return nil
	})
	if called || a.dockIconID != "" || a.dockIconPref != "" || a.dockIconError != "" {
		t.Fatalf("unexpected restore on default: called=%v id=%q pref=%q err=%q", called, a.dockIconID, a.dockIconPref, a.dockIconError)
	}

	// Saved custom icon should be applied on startup.
	a.dockIconPref = "match"
	var applied []byte
	a.restoreDockIconWith(func(data []byte) error {
		applied = data
		return nil
	})
	if a.dockIconID != "match" || a.dockIconPref != "match" || a.dockIconError != "" || !bytes.Equal(applied, matchIcon) {
		t.Fatalf("restore match failed: id=%q pref=%q err=%q", a.dockIconID, a.dockIconPref, a.dockIconError)
	}

	// Startup restore failure should reflect that the bundle icon remains visible
	// while preserving the saved preference across shutdown (even if other settings change).
	a.dockIconPref = "pancake"
	initial := a.preferences()
	a.restoreDockIconWith(func([]byte) error {
		return errors.New("decode failed")
	})
	if a.dockIconID != "" || a.dockIconPref != "pancake" || a.dockIconError == "" {
		t.Fatalf("failed restore state mismatch: id=%q pref=%q err=%q", a.dockIconID, a.dockIconPref, a.dockIconError)
	}
	if final := a.preferences(); final != initial {
		t.Fatalf("failed restore mutated saved preferences: got %+v, want %+v", final, initial)
	}
	a.appearance = "Dark"
	if final := a.preferences(); final.DockIcon != "pancake" {
		t.Fatalf("unrelated setting change lost saved DockIcon preference: got %q", final.DockIcon)
	}

	// Explicitly selecting Default after a failed restore should clear the error and update the saved preference.
	var clearedTo []byte = []byte("sentinel")
	a.chooseDockIcon(dockIcons[0], func(data []byte) error {
		clearedTo = data
		return nil
	})
	if clearedTo != nil || a.dockIconID != "" || a.dockIconPref != "" || a.dockIconError != "" {
		t.Fatalf("explicit default after failed restore failed: id=%q pref=%q err=%q", a.dockIconID, a.dockIconPref, a.dockIconError)
	}

	// Retrying the preferred icon after a failed restore should apply it once SetIcon succeeds.
	a.dockIconPref = "pancake"
	a.restoreDockIconWith(func([]byte) error {
		return errors.New("temporary failure")
	})
	a.chooseDockIcon(dockIconForID("pancake"), func(data []byte) error {
		applied = data
		return nil
	})
	if a.dockIconID != "pancake" || a.dockIconPref != "pancake" || a.dockIconError != "" || !bytes.Equal(applied, pancakeIcon) {
		t.Fatalf("retry after failed restore failed: id=%q pref=%q err=%q", a.dockIconID, a.dockIconPref, a.dockIconError)
	}
}

func TestUnrecognizedOrMissingDockIconUsesDefault(t *testing.T) {
	for _, tc := range []struct {
		data string
		want string
	}{
		{`{"Version":1,"Site":"leetcode.com","Theme":"Dark"}`, ""},
		{`{"Version":1,"Site":"leetcode.com","Theme":"Dark","DockIcon":"retired"}`, ""},
		{`{"Version":1,"Site":"leetcode.com","Theme":"Dark","DockIcon":"match"}`, "match"},
	} {
		a, _ := testWorkspace(t)
		path := filepath.Join(t.TempDir(), "ui-preferences.json")
		if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := a.readPreferences(path); err != nil || a.dockIconPref != tc.want || a.dockIconID != "" || a.appearance != "Dark" {
			t.Fatalf("unexpected preferences pref=%q (want %q), active=%q, appearance=%q, err=%v", a.dockIconPref, tc.want, a.dockIconID, a.appearance, err)
		}
	}
}
