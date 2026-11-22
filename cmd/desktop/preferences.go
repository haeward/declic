package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/haeward/declic/internal/leetcode"
)

type preferences struct {
	Version                                                            int
	Site                                                               leetcode.Site
	Theme, Language                                                    string
	DockIcon                                                           string `json:",omitempty"`
	OpenOnLaunch                                                       string `json:",omitempty"`
	LastProblemSlug                                                    string `json:",omitempty"`
	NoRestoreSession                                                   bool   `json:",omitempty"`
	Contrast                                                           float64
	DescriptionSize, PageSize, EditorFontSize, EditorTabSize           int
	Compact, Topics, HidePremium, HideAcceptance, HideDifficulty, Wrap bool
	NoAutoClose                                                        bool `json:",omitempty"`
	SidebarCollapsed, ProblemsCollapsed, WorkspaceCollapsed            bool
	DailyCollapsed                                                     bool
	Split, EditorSplit                                                 float32
}

func (a *app) preferences() preferences {
	return preferences{Version: 1, Site: a.site, Theme: a.appearance, Language: a.language, DockIcon: a.dockIconPref, OpenOnLaunch: a.openOnLaunch, LastProblemSlug: a.lastProblemSlug, NoRestoreSession: a.noRestoreSession, Contrast: a.contrast, DescriptionSize: a.descriptionSize, PageSize: a.perPage(), EditorFontSize: a.editorFontSize, EditorTabSize: a.editorTabSize, Compact: a.compact, Topics: a.showTopics, HidePremium: a.hidePremium, HideAcceptance: a.hideAcceptance, HideDifficulty: a.hideDifficulty, Wrap: a.code.SoftWrap, NoAutoClose: a.code.NoAutoClose, SidebarCollapsed: a.sidebarCollapsed, ProblemsCollapsed: a.sidebarCollapsed, WorkspaceCollapsed: a.sidebarCollapsed, DailyCollapsed: a.dailyCollapsed, Split: a.split, EditorSplit: a.editorSplit}
}
func (a *app) readPreferences(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	p := a.preferences()
	if err = json.Unmarshal(data, &p); err != nil {
		return err
	}
	var legacy struct {
		SidebarCollapsed *bool
	}
	_ = json.Unmarshal(data, &legacy)
	if p.Version != 1 || p.Site != leetcode.COM && p.Site != leetcode.CN {
		return fmt.Errorf("unsupported preferences file")
	}
	a.site, a.appearance, a.contrast = p.Site, p.Theme, max(0, min(1, p.Contrast))
	a.dockIconPref = dockIconForID(p.DockIcon).id
	if p.OpenOnLaunch == "last_workspace" {
		a.openOnLaunch = "last_workspace"
	} else {
		a.openOnLaunch = ""
	}
	a.lastProblemSlug = p.LastProblemSlug
	a.noRestoreSession = p.NoRestoreSession
	if p.Language != "" {
		a.language = p.Language
	}
	a.descriptionSize = max(13, min(19, p.DescriptionSize))
	if p.EditorFontSize >= 11 && p.EditorFontSize <= 18 {
		a.editorFontSize = p.EditorFontSize
	}
	if p.EditorTabSize == 0 || p.EditorTabSize == 2 || p.EditorTabSize == 4 {
		a.editorTabSize = p.EditorTabSize
	}
	if p.PageSize == 20 || p.PageSize == 50 || p.PageSize == 100 {
		a.pageSize = p.PageSize
	}
	a.compact, a.showTopics, a.hidePremium, a.hideAcceptance, a.hideDifficulty, a.code.SoftWrap, a.code.NoAutoClose = p.Compact, p.Topics, p.HidePremium, p.HideAcceptance, p.HideDifficulty, p.Wrap, p.NoAutoClose
	if legacy.SidebarCollapsed != nil {
		a.sidebarCollapsed = *legacy.SidebarCollapsed
	} else {
		a.sidebarCollapsed = p.ProblemsCollapsed
	}
	a.dailyCollapsed = p.DailyCollapsed
	a.split, a.editorSplit = p.Split, p.EditorSplit
	return nil
}
func (p preferences) write(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".preferences-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = json.NewEncoder(f).Encode(p); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func (a *app) maybePersistPreferences(now time.Time) {
	if a.closed || a.prefPath == "" || a.prefReadErr != nil || a.prefSaving || a.post == nil {
		return
	}
	cur := a.preferences()
	if cur == a.lastSavedPrefs {
		a.prefDirty = false
		return
	}
	if !a.prefDirty {
		a.prefDirty = true
		a.prefDirtyAt = now
		return
	}
	if now.Sub(a.prefDirtyAt) < 600*time.Millisecond {
		return
	}
	a.prefSaving = true
	path := a.prefPath
	go func() {
		err := cur.write(path)
		a.post(func() {
			a.prefSaving = false
			if err != nil {
				log.Print("Saving preferences: ", err)
				a.prefDirtyAt = time.Now()
				return
			}
			a.lastSavedPrefs = cur
			if a.preferences() == cur {
				a.prefDirty = false
			} else {
				a.prefDirtyAt = time.Now()
			}
		})
	}()
}
