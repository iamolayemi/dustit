// Package e2e runs the real dustit binary against a fake home folder and checks
// that exactly the right files are removed (and that undo puts them back).
package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "dustit-e2e-bin")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "dustit")
	if out, err := exec.Command("go", "build", "-o", bin, "..").CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// home is a throwaway home folder.
type home struct {
	t    *testing.T
	root string
}

func newHome(t *testing.T) *home {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := &home{t, root}
	h.mkdir(".Trash")
	return h
}

func (h *home) path(rel string) string { return filepath.Join(h.root, rel) }

func (h *home) mkdir(rel string) {
	if err := os.MkdirAll(h.path(rel), 0o755); err != nil {
		h.t.Fatal(err)
	}
}

func (h *home) write(rel string, size int, age time.Duration) {
	h.t.Helper()
	p := h.path(rel)
	h.mkdir(filepath.Dir(rel))
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		h.t.Fatal(err)
	}
	if age > 0 {
		old := time.Now().Add(-age)
		_ = os.Chtimes(p, old, old)
	}
}

func (h *home) age(rel string, age time.Duration) {
	old := time.Now().Add(-age)
	_ = os.Chtimes(h.path(rel), old, old)
}

func (h *home) exists(rel string) bool {
	_, err := os.Lstat(h.path(rel))
	return err == nil
}

func (h *home) run(args ...string) string {
	h.t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME="+h.root, "NO_COLOR=1", "DUSTIT_CONFIG="+h.path(".config/dustit/config.yaml"))
	cmd.Stdin = strings.NewReader("")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output() // stdout only: progress messages go to stderr
	if err != nil {
		h.t.Fatalf("dustit %s: %v\n%s%s", strings.Join(args, " "), err, out, stderr.String())
	}
	return string(out)
}

const year = 365 * 24 * time.Hour

func seed(h *home) {
	h.write("Library/Caches/com.example.app/cache.db", 50_000, 0)
	h.write(".npm/_cacache/content-v2/blob", 80_000, 0)
	h.write("Documents/important.txt", 1_000, 0)
	h.write("Documents/keep/notes.md", 1_000, 0)
	// a cache entry that is a symlink to personal files: only the link may go
	if err := os.Symlink(h.path("Documents/keep"), h.path("Library/Caches/link-to-documents")); err != nil {
		h.t.Fatal(err)
	}
	// an idle project and an active one
	h.write("Projects/idle/package.json", 10, year)
	h.write("Projects/idle/src/index.js", 10, year)
	h.write("Projects/idle/node_modules/dep/index.js", 30_000, 0)
	h.age("Projects/idle/src", year)
	h.age("Projects/idle", year)
	h.write("Projects/active/package.json", 10, 0)
	h.write("Projects/active/node_modules/dep/index.js", 30_000, 0)
	// duplicates (6 MB, above the 5 MB default)
	h.write("Documents/report.pdf", 6_000_000, 0)
	time.Sleep(10 * time.Millisecond) // the copy is created later, so the original is kept
	h.write("Desktop/report copy.pdf", 6_000_000, 0)
}

func TestScanJSON(t *testing.T) {
	h := newHome(t)
	seed(h)
	var res struct {
		Categories []struct {
			ID        string `json:"id"`
			Bytes     int64  `json:"size_bytes"`
			ItemCount int    `json:"item_count"`
		} `json:"categories"`
	}
	if err := json.Unmarshal([]byte(h.run("scan", "--json")), &res); err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, c := range res.Categories {
		got[c.ID] = c.ItemCount
	}
	for id, want := range map[string]int{"user-caches": 2, "npm-cache": 1, "stale-deps": 1, "duplicates": 1} {
		if got[id] != want {
			t.Errorf("%s: %d items, want %d", id, got[id], want)
		}
	}
}

func TestCleanRemovesOnlyWhatItShould(t *testing.T) {
	h := newHome(t)
	seed(h)
	h.run("clean", "--only", "user-caches,npm-cache,stale-deps", "--yes")

	for _, gone := range []string{"Library/Caches/com.example.app", ".npm/_cacache/content-v2", "Projects/idle/node_modules", "Library/Caches/link-to-documents"} {
		if h.exists(gone) {
			t.Errorf("%s should have been removed", gone)
		}
	}
	for _, kept := range []string{"Library/Caches", ".npm/_cacache", "Documents/important.txt", "Documents/keep/notes.md",
		"Projects/active/node_modules/dep/index.js", "Projects/idle/package.json", "Projects/idle/src/index.js"} {
		if !h.exists(kept) {
			t.Errorf("%s must survive", kept)
		}
	}
}

func TestDuplicatesTrashAndUndo(t *testing.T) {
	h := newHome(t)
	seed(h)
	h.run("clean", "--only", "duplicates", "--yes")
	if !h.exists("Documents/report.pdf") || h.exists("Desktop/report copy.pdf") {
		t.Fatal("the later copy should go to the Trash and the original stay")
	}
	if !h.exists(".Trash/report copy.pdf") {
		t.Fatal("the copy should be in the Trash, not deleted")
	}
	out := h.run("history")
	if !strings.Contains(out, "1 trashed") {
		t.Errorf("history should show the trashed item:\n%s", out)
	}
	h.run("undo", "--yes")
	if !h.exists("Desktop/report copy.pdf") || h.exists(".Trash/report copy.pdf") {
		t.Fatal("undo should put the copy back")
	}
}

func TestConfigExcludeIsHonoured(t *testing.T) {
	h := newHome(t)
	seed(h)
	h.mkdir(".config/dustit")
	cfg := "exclude:\n  - ~/Projects/idle/node_modules\n  - ~/Library/Caches/com.example.app\n"
	if err := os.WriteFile(h.path(".config/dustit/config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := h.run("config", "check"); !strings.Contains(out, "valid") {
		t.Fatalf("config check: %s", out)
	}
	h.run("clean", "--only", "user-caches,stale-deps", "--yes")
	if !h.exists("Projects/idle/node_modules/dep/index.js") || !h.exists("Library/Caches/com.example.app/cache.db") {
		t.Fatal("excluded paths must never be removed")
	}
}

func TestInstallersAndScreenshots(t *testing.T) {
	h := newHome(t)
	month := 40 * 24 * time.Hour
	h.write("Downloads/Some App 2.1.dmg", 2_000, month)
	h.write("Downloads/fresh.dmg", 2_000, 0)
	h.write("Desktop/Screenshot 2026-01-02 at 10.00.00.png", 3_000, month)
	h.write("Desktop/holiday.png", 3_000, month)
	h.run("clean", "--only", "installers,screenshots", "--yes")
	if h.exists("Downloads/Some App 2.1.dmg") || h.exists("Desktop/Screenshot 2026-01-02 at 10.00.00.png") {
		t.Error("old installers and screenshots should go to the Trash")
	}
	if !h.exists(".Trash/Some App 2.1.dmg") {
		t.Error("installers must be trashed, not deleted")
	}
	if !h.exists("Downloads/fresh.dmg") || !h.exists("Desktop/holiday.png") {
		t.Error("new installers and ordinary pictures must stay")
	}
}

func TestMigratesDataFromOldName(t *testing.T) {
	h := newHome(t)
	h.write("Library/Application Support/macclean/history.jsonl", 0, 0)
	if err := os.WriteFile(h.path("Library/Application Support/macclean/history.jsonl"),
		[]byte(`{"run":"20260101-000000.000","command":"macclean clean","action":"trashed","path":"/x","trash_path":"/y","size":5}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "history", "--json")
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin", "HOME="+h.root, "NO_COLOR=1", "XDG_CONFIG_HOME=")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !h.exists("Library/Application Support/dustit/history.jsonl") || h.exists("Library/Application Support/macclean") {
		t.Fatal("history should move to the new folder")
	}
	if !strings.Contains(string(out), "20260101-000000.000") {
		t.Errorf("migrated history should be readable: %s", out)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	h := newHome(t)
	seed(h)
	h.run("clean", "--all", "--yes", "--dry-run")
	for _, p := range []string{"Library/Caches/com.example.app/cache.db", ".npm/_cacache/content-v2/blob", "Projects/idle/node_modules", "Desktop/report copy.pdf"} {
		if !h.exists(p) {
			t.Errorf("dry run removed %s", p)
		}
	}
}
