package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Key struct{ Site, Account, Problem, Language string }

var (
	segment            = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	languageExtensions = map[string]string{
		"cpp":        "cpp",
		"java":       "java",
		"python3":    "py",
		"python":     "py",
		"javascript": "js",
		"typescript": "ts",
		"csharp":     "cs",
		"c":          "c",
		"golang":     "go",
		"kotlin":     "kt",
		"swift":      "swift",
		"rust":       "rs",
		"ruby":       "rb",
		"php":        "php",
		"dart":       "dart",
		"scala":      "scala",
		"elixir":     "ex",
		"erlang":     "erl",
		"racket":     "rkt",
		"mysql":      "sql",
		"mssql":      "sql",
		"oraclesql":  "sql",
		"postgresql": "sql",
		"pythondata": "py",
		"bash":       "sh",
	}
)

type Store struct{ Root string }

func (s Store) path(k Key) (string, error) {
	if strings.TrimSpace(s.Root) == "" {
		return "", fmt.Errorf("missing workspace root")
	}
	for _, part := range []string{k.Site, k.Account, k.Problem, k.Language} {
		if !segment.MatchString(part) {
			return "", fmt.Errorf("invalid workspace key")
		}
	}
	ext := Extension(k.Language)
	if ext == "" {
		return "", fmt.Errorf("unsupported language")
	}
	return filepath.Join(s.Root, k.Site, k.Account, k.Problem, k.Language, "solution."+ext), nil
}

// LoadDraft distinguishes an untouched template from a draft actually on disk.
func (s Store) LoadDraft(k Key, template string) (string, bool, error) {
	path, err := s.path(k)
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return template, false, nil
	}
	return string(data), err == nil, err
}

// The UI serializes writes. A failed replacement preserves the existing draft.
func (s Store) Save(k Key, text string) error {
	path, err := s.path(k)
	if err != nil {
		return err
	}
	return writeAtomic(path, text)
}

func writeAtomic(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".draft-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.WriteString(text); err != nil {
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

// Extension returns an empty string for unsupported editor languages.
func Extension(language string) string {
	return languageExtensions[language]
}
