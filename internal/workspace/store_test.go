package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDraftIsolationAndEmptyRecovery(t *testing.T) {
	s := Store{Root: t.TempDir()}
	keys := []Key{{"leetcode-cn", "anonymous", "1", "java"}, {"leetcode-com", "anonymous", "1", "java"}, {"leetcode-cn", "user", "1", "java"}, {"leetcode-cn", "anonymous", "1", "rust"}, {"leetcode-cn", "anonymous", "1", "golang"}, {"leetcode-cn", "anonymous", "1", "cpp"}, {"leetcode-cn", "anonymous", "1", "javascript"}}
	if text, persisted, err := s.LoadDraft(keys[0], "template"); err != nil || persisted || text != "template" {
		t.Fatalf("unsaved template should report persisted=false: %q %v %v", text, persisted, err)
	}
	for i, k := range keys {
		if err := s.Save(k, string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	for i, k := range keys {
		text, persisted, err := (Store{Root: s.Root}).LoadDraft(k, "template")
		if err != nil || !persisted || text != string(rune('a'+i)) {
			t.Fatalf("cross-key or restart loss: %q %v %v", text, persisted, err)
		}
	}
	if err := s.Save(keys[0], ""); err != nil {
		t.Fatal(err)
	}
	if text, persisted, err := s.LoadDraft(keys[0], "template"); err != nil || !persisted || text != "" {
		t.Fatal("empty draft overwritten", persisted, err)
	}
	if err := (Store{}).Save(keys[0], "x"); err == nil {
		t.Fatal("accepted empty workspace root")
	}
	bad := keys[0]
	bad.Problem = "../escape"
	if err := s.Save(bad, "x"); err == nil {
		t.Fatal("accepted unsafe path")
	}
	for lang, want := range map[string]string{"kotlin": "kt", "swift": "swift", "ruby": "rb", "php": "php", "mysql": "sql", "bash": "sh"} {
		if got := Extension(lang); got != want {
			t.Fatalf("Extension(%q)=%q, want %q", lang, got, want)
		}
	}
}

func TestStorageFailureIsVisible(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	s := Store{Root: root}
	k := Key{"leetcode-cn", "anonymous", "1", "java"}
	if err := s.Save(k, "replacement"); err == nil {
		t.Fatal("save failure hidden")
	}
	if _, _, err := s.LoadDraft(k, "template"); err == nil {
		t.Fatal("read failure hidden")
	}
	data, _ := os.ReadFile(root)
	if string(data) != "keep" {
		t.Fatal("damaged existing data")
	}
}
