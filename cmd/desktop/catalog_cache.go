package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/haeward/declic/internal/leetcode"
)

func catalogCachePath(root, site string) string {
	cacheDir := publicCacheDir(root)
	if cacheDir == "" || (site != string(leetcode.COM) && site != string(leetcode.CN)) {
		return ""
	}
	return filepath.Join(cacheDir, "catalog", site+".json")
}

func readCatalogCacheFile(path string) ([]leetcode.Problem, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	var rows []leetcode.Problem
	if json.NewDecoder(f).Decode(&rows) != nil {
		return nil, false
	}
	for i := range rows {
		rows[i].Normalize()
	}
	info, err := f.Stat()
	return rows, err == nil && time.Since(info.ModTime()) < 24*time.Hour
}

// Catalog cache contains only public metadata and can be rebuilt from the site.
func readCatalogCache(root, site string) ([]leetcode.Problem, bool) {
	path := catalogCachePath(root, site)
	if path == "" {
		return nil, false
	}
	if rows, fresh := readCatalogCacheFile(path); rows != nil {
		return rows, fresh
	}
	// Read-only fallback for legacy caches previously stored under Workspace/cache/.
	legacy := filepath.Join(root, "cache", site+".json")
	if legacy != path {
		return readCatalogCacheFile(legacy)
	}
	return nil, false
}

func writeCatalogCache(root, site string, rows []leetcode.Problem) {
	path := catalogCachePath(root, site)
	if path == "" {
		return
	}
	if raw, err := json.Marshal(rows); err == nil {
		writeCacheFile(path, raw)
	}
}
