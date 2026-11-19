package finder

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, p string, data []byte, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-age)
	_ = os.Chtimes(p, old, old)
}

func ageDirs(t *testing.T, root string, age time.Duration, dirs ...string) {
	old := time.Now().Add(-age)
	for _, d := range dirs {
		_ = os.Chtimes(filepath.Join(root, d), old, old)
	}
}

func TestStaleDeps(t *testing.T) {
	root := t.TempDir()
	year := 365 * 24 * time.Hour
	writeFile(t, filepath.Join(root, "idle/package.json"), []byte("{}"), year)
	writeFile(t, filepath.Join(root, "idle/src/index.js"), []byte("x"), year)
	writeFile(t, filepath.Join(root, "idle/node_modules/a/index.js"), []byte("x"), 0)
	writeFile(t, filepath.Join(root, "active/package.json"), []byte("{}"), year)
	writeFile(t, filepath.Join(root, "active/src/index.js"), []byte("x"), 0)
	writeFile(t, filepath.Join(root, "active/node_modules/a/index.js"), []byte("x"), 0)
	writeFile(t, filepath.Join(root, "php/composer.json"), []byte("{}"), year)
	writeFile(t, filepath.Join(root, "php/vendor/autoload.php"), []byte("x"), year)
	writeFile(t, filepath.Join(root, "gomod/go.mod"), []byte("module x"), year)
	writeFile(t, filepath.Join(root, "gomod/vendor/x.go"), []byte("x"), year) // Go vendor: not a dep folder
	ageDirs(t, root, year, "idle/src", "idle", "active/src", "php", "gomod/vendor", "gomod")

	got := map[string]bool{}
	for _, it := range StaleDeps([]string{root}, 30) {
		got[it.Path] = true
	}
	want := []string{filepath.Join(root, "idle/node_modules"), filepath.Join(root, "php/vendor")}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %s", w)
		}
	}
}

func TestDupes(t *testing.T) {
	root := t.TempDir()
	a := make([]byte, 2<<20)
	for i := range a {
		a[i] = byte(i % 251)
	}
	b := append([]byte(nil), a...)
	b[len(b)/2] ^= 1 // same size and same head/tail, different middle
	writeFile(t, filepath.Join(root, "orig.bin"), a, 48*time.Hour)
	writeFile(t, filepath.Join(root, "sub/copy.bin"), a, 0)
	writeFile(t, filepath.Join(root, "other.bin"), b, 0)
	writeFile(t, filepath.Join(root, "Cache/cached.bin"), a, 0) // cache folders are skipped

	groups := Dupes([]string{root}, 1<<20)
	if len(groups) != 1 || len(groups[0].Paths) != 2 {
		t.Fatalf("want one group of two, got %+v", groups)
	}
	extras := DupeExtras(groups)
	if len(extras) != 1 || extras[0].Path != filepath.Join(root, "sub/copy.bin") {
		t.Fatalf("want the newer copy as the extra, got %+v", extras)
	}
}

func TestBelongsToInstalled(t *testing.T) {
	ids := map[string]bool{"com.docker.docker": true, "com.tinyspeck.slackmacgap": true}
	for id, want := range map[string]bool{
		"com.docker.docker":            true,
		"com.docker.docker.helper":     true,
		"com.docker.vmnetd":            true, // same vendor
		"com.tinyspeck.slackmacgap.sh": true,
		"com.spotify.client":           false,
	} {
		if got := BelongsToInstalled(id, ids); got != want {
			t.Errorf("BelongsToInstalled(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestJetBrainsVersionPattern(t *testing.T) {
	m := jbRe.FindStringSubmatch("PhpStorm2026.1-backup")
	if m == nil || m[1] != "PhpStorm" || m[2] != "2026" || m[3] != "1" || m[4] != "-backup" {
		t.Fatalf("bad match %v", m)
	}
}

func TestDedupeRoots(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "a/b"), 0o755)
	got := DedupeRoots([]string{filepath.Join(root, "a"), filepath.Join(root, "a/b"), filepath.Join(root, "missing")})
	if len(got) != 1 {
		t.Fatalf("want only the outer root, got %v", got)
	}
}
