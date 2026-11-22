package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

type storageStats struct {
	draftCount int
	draftBytes int64
	cacheBytes int64
}

func scanStorageStats(workspaceRoot string) storageStats {
	var st storageStats
	if workspaceRoot != "" {
		_ = filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if strings.HasPrefix(d.Name(), "solution.") {
				st.draftCount++
				if info, statErr := d.Info(); statErr == nil {
					st.draftBytes += info.Size()
				}
			}
			return nil
		})
	}
	if cacheDir := publicCacheDir(workspaceRoot); cacheDir != "" {
		_ = filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, statErr := d.Info(); statErr == nil {
				st.cacheBytes += info.Size()
			}
			return nil
		})
	}
	return st
}

// clearPublicCaches removes only replaceable public metadata under <user-data>/cache/.
// It never modifies user drafts under Workspace/ or ui-preferences.json.
func clearPublicCaches(workspaceRoot string) error {
	cacheDir := publicCacheDir(workspaceRoot)
	if cacheDir == "" || filepath.Base(cacheDir) != "cache" {
		return nil
	}
	entries, err := os.ReadDir(cacheDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(cacheDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func formatByteSize(n int64) string {
	switch {
	case n <= 0:
		return "0 B"
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func (a *app) ensureStorageStats() {
	if a.storageLoaded || a.storageScanning || a.store.Root == "" || a.post == nil {
		return
	}
	a.refreshStorageStats()
}

func (a *app) refreshStorageStats() {
	if a.storageScanning || a.store.Root == "" || a.post == nil {
		return
	}
	a.storageScanning = true
	root := a.store.Root
	go func() {
		stats := scanStorageStats(root)
		a.post(func() {
			if a.closed {
				return
			}
			a.storageScanning = false
			a.storageLoaded = true
			a.storageStats = stats
			if a.settingsWindow != nil {
				a.settingsWindow.Update(func() {})
			}
		})
	}()
}

func (a *app) openWorkspaceFolder() {
	if a.store.Root == "" {
		return
	}
	if err := os.MkdirAll(a.store.Root, 0700); err != nil {
		a.storageNotice = "Could not open drafts folder: " + err.Error()
		return
	}
	if err := mygo.Shell.OpenPath(a.store.Root); err != nil {
		a.storageNotice = "Could not open drafts folder: " + err.Error()
		return
	}
	a.storageNotice = ""
}

func (a *app) clearPublicCachesAsync() {
	if a.storageClearing || a.busy || a.store.Root == "" || a.post == nil {
		return
	}
	a.storageClearing = true
	a.storageNotice = ""
	root := a.store.Root
	go func() {
		err := clearPublicCaches(root)
		stats := scanStorageStats(root)
		a.post(func() {
			if a.closed {
				return
			}
			a.storageClearing = false
			a.storageLoaded = true
			a.storageStats = stats
			if err != nil {
				a.storageNotice = "Could not clear caches: " + err.Error()
			} else {
				a.detailCache = nil
				a.resourceCache = nil
				a.slideDecks = nil
				a.linkedImages = nil
				a.cnVideos = nil
				a.playgrounds = nil
				imageCacheMu.Lock()
				clear(imageCache)
				imageCacheMu.Unlock()
				a.storageNotice = "Public caches cleared. Local drafts were not affected."
			}
			if a.settingsWindow != nil {
				a.settingsWindow.Update(func() {})
			}
		})
	}()
}
