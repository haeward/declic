package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/haeward/declic/internal/leetcode"
	"github.com/haeward/declic/internal/render"
	_ "golang.org/x/image/webp"
)

const publicDetailTTL = 7 * 24 * time.Hour

type cachedDetail struct {
	Updated time.Time        `json:"updated"`
	Problem leetcode.Problem `json:"problem"`
}

type cachedEditorial struct {
	Updated time.Time         `json:"updated"`
	Article *leetcode.Article `json:"article"`
}

func publicCacheDir(root string) string {
	if root == "" {
		return ""
	}
	if filepath.Base(root) == "Workspace" {
		return filepath.Join(filepath.Dir(root), "cache")
	}
	return filepath.Join(root, "cache")
}

func detailCachePath(root, site, kind, slug string) string {
	cacheDir := publicCacheDir(root)
	if cacheDir == "" || (site != string(leetcode.COM) && site != string(leetcode.CN)) || slug == "" || !render.IsSafeSlug(slug) {
		return ""
	}
	return filepath.Join(cacheDir, strings.ReplaceAll(site, ".", "-"), kind, slug+".json")
}

func writeCacheFile(path string, raw []byte) {
	dir := filepath.Dir(path)
	if os.MkdirAll(dir, 0700) != nil {
		return
	}
	f, err := os.CreateTemp(dir, ".cache-*")
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	_, err = f.Write(raw)
	closeErr := f.Close()
	if err == nil && closeErr == nil {
		_ = os.Rename(f.Name(), path)
	}
}

func readDetailCache(root, site, slug string) (leetcode.Problem, bool) {
	path := detailCachePath(root, site, "problems", slug)
	if path == "" {
		return leetcode.Problem{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return leetcode.Problem{}, false
	}
	var c cachedDetail
	if json.Unmarshal(raw, &c) != nil || c.Problem.ID == "" || c.Problem.Paid || time.Since(c.Updated) > publicDetailTTL {
		return leetcode.Problem{}, false
	}
	c.Problem.Normalize()
	return c.Problem, true
}

func writeDetailCache(root, site string, p leetcode.Problem) {
	if p.Paid || p.ID == "" || p.Slug == "" {
		return
	}
	path := detailCachePath(root, site, "problems", p.Slug)
	if path == "" {
		return
	}
	raw, err := json.Marshal(cachedDetail{Updated: time.Now(), Problem: p})
	if err == nil {
		writeCacheFile(path, raw)
	}
}

func readEditorialCache(root, site, slug string) (*leetcode.Article, bool) {
	path := detailCachePath(root, site, "editorials", slug)
	if path == "" {
		return nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var c cachedEditorial
	if json.Unmarshal(raw, &c) != nil || time.Since(c.Updated) > publicDetailTTL {
		return nil, false
	}
	if c.Article != nil && c.Article.Paid {
		return nil, false
	}
	return c.Article, true
}

func writeEditorialCache(root, site, slug string, art *leetcode.Article) {
	if art != nil && (art.Paid || !art.Visible) {
		return
	}
	path := detailCachePath(root, site, "editorials", slug)
	if path == "" {
		return
	}
	raw, err := json.Marshal(cachedEditorial{Updated: time.Now(), Article: art})
	if err == nil {
		writeCacheFile(path, raw)
	}
}

var (
	imageCacheMu sync.Mutex
	imageCache   = map[string]*ui.Bitmap{}
	imageHTTP    = &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
)

// fetchSafeBitmap downloads a public HTTPS image without credentials and caches it in memory.
func fetchSafeBitmap(rawURL string) (*ui.Bitmap, error) {
	cleaned := strings.TrimSpace(rawURL)
	if strings.HasPrefix(cleaned, "//") {
		cleaned = "https:" + cleaned
	}
	u, err := url.Parse(cleaned)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("unsupported image URL")
	}
	imageCacheMu.Lock()
	if bmp, ok := imageCache[cleaned]; ok {
		imageCacheMu.Unlock()
		return bmp, nil
	}
	imageCacheMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cleaned, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Safari/537.36")
	req.Header.Set("Accept", "image/png,image/jpeg,image/webp,image/gif,*/*;q=0.8")
	resp, err := imageHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bmp := ui.NewBitmap(img)
	imageCacheMu.Lock()
	if len(imageCache) > 128 {
		clear(imageCache)
	}
	imageCache[cleaned] = bmp
	imageCacheMu.Unlock()
	return bmp, nil
}
