package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDraftIsolationAndEmptyRecovery(t *testing.T) {
	s := Store{Root: t.TempDir()}
	keys := []Key{{"leetcode-cn", "anonymous", "1", "java"}, {"leetcode-com", "anonymous", "1", "java"}, {"leetcode-cn", "user", "1", "java"}, {"leetcode-cn", "anonymous", "1", "rust"}, {"leetcode-cn", "anonymous", "1", "golang"}, {"leetcode-cn", "anonymous", "1", "cpp"}, {"leetcode-cn", "anonymous", "1", "javascript"}}
	for i, k := range keys {
		if err := s.Save(k, string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	for i, k := range keys {
		text, err := (Store{Root: s.Root}).Load(k, "template")
		if err != nil || text != string(rune('a'+i)) {
			t.Fatalf("cross-key or restart loss: %q %v", text, err)
		}
	}
	if err := s.Save(keys[0], ""); err != nil {
		t.Fatal(err)
	}
	if text, err := s.Load(keys[0], "template"); err != nil || text != "" {
		t.Fatal("empty draft overwritten", err)
	}
	bad := keys[0]
	bad.Problem = "../escape"
	if err := s.Save(bad, "x"); err == nil {
		t.Fatal("accepted unsafe path")
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
	if _, err := s.Load(k, "template"); err == nil {
		t.Fatal("read failure hidden")
	}
	data, _ := os.ReadFile(root)
	if string(data) != "keep" {
		t.Fatal("damaged existing data")
	}
}
