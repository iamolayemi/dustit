package finder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchesPin(t *testing.T) {
	for _, c := range []struct {
		name, pin string
		want      bool
	}{
		{"v22.14.0", "22", true},
		{"v22.14.0", "v22.14.0", true},
		{"v22.14.0", "22.14", true},
		{"v24.18.0", "22", false},
		{"3.10.13", "3.10", true},
		{"3.1.4", "3.10", false},
		{"8.1", "^8.1", true},
		{"stable-aarch64-apple-darwin", "stable-aarch64-apple-darwin", true},
		{"v22.14.0", "lts/*", false},
	} {
		if got := matchesPin(c.name, c.pin); got != c.want {
			t.Errorf("matchesPin(%q, %q) = %v, want %v", c.name, c.pin, got, c.want)
		}
	}
}

func TestVersionToolKeepsDefaultNewestAndPinned(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"v18.20.0", "v20.11.0", "v22.14.0", "v24.18.0", "v25.9.0"} {
		_ = os.MkdirAll(filepath.Join(dir, v), 0o755)
	}
	tool := versionTool{Tool: "node", Manager: "nvm", Dirs: []string{dir}, Default: func() []string { return []string{"v24.18.0"} }}
	got := map[string]bool{}
	for _, it := range tool.old(map[string][]string{"node": {"20"}}) {
		got[filepath.Base(it.Path)] = true
	}
	// 25.9 newest, 24.18 default, 20.11 pinned → only 18 and 22 go
	if len(got) != 2 || !got["v18.20.0"] || !got["v22.14.0"] {
		t.Fatalf("got %v", got)
	}
}

func TestVersionToolPerMajor(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"corretto-17.0.8", "corretto-17.0.9", "openjdk-21.0.1", "openjdk-21.0.2"} {
		_ = os.MkdirAll(filepath.Join(dir, v), 0o755)
	}
	tool := versionTool{Tool: "java", Manager: "JDKs", Dirs: []string{dir}, PerMajor: true}
	var names []string
	for _, it := range tool.old(nil) {
		names = append(names, filepath.Base(it.Path))
	}
	if strings.Join(names, ",") != "openjdk-21.0.1,corretto-17.0.8" {
		t.Fatalf("got %v", names)
	}
}

func TestRustChannelsKept(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"stable-aarch64-apple-darwin", "nightly-2024-01-01-aarch64-apple-darwin", "1.75.0-aarch64-apple-darwin"} {
		_ = os.MkdirAll(filepath.Join(dir, v), 0o755)
	}
	var tool versionTool
	for _, vt := range versionTools {
		if vt.Manager == "rustup" {
			tool = vt
		}
	}
	tool.Dirs = []string{dir}
	tool.Default = func() []string { return []string{"stable-aarch64-apple-darwin"} }
	got := map[string]bool{}
	for _, it := range tool.old(map[string][]string{"rust": {"1.75"}}) {
		got[filepath.Base(it.Path)] = true
	}
	if len(got) != 1 || !got["nightly-2024-01-01-aarch64-apple-darwin"] {
		t.Fatalf("only the old dated nightly should go, got %v", got)
	}
}

func TestProjectPins(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "web/.nvmrc"), []byte("20\n"), 0)
	writeFile(t, filepath.Join(root, "api/composer.json"), []byte(`{"require":{"php":"^8.1|^8.2"}}`), 0)
	writeFile(t, filepath.Join(root, "ml/.tool-versions"), []byte("python 3.11.4\nnodejs 18.19.0\n"), 0)
	writeFile(t, filepath.Join(root, "app/gradle/wrapper/gradle-wrapper.properties"), []byte("distributionUrl=https\\://services.gradle.org/distributions/gradle-8.5-bin.zip"), 0)
	writeFile(t, filepath.Join(root, "web/node_modules/x/.nvmrc"), []byte("12"), 0) // inside deps: ignored
	pins := ProjectPins([]string{root})
	want := map[string]string{"node": "20,18.19.0", "php": "8.1", "python": "3.11.4", "gradle": "8.5"}
	for tool, w := range want {
		got := strings.Join(pins[tool], ",")
		for _, v := range strings.Split(w, ",") {
			if !strings.Contains(got, v) {
				t.Errorf("%s pins = %q, missing %s", tool, got, v)
			}
		}
	}
	if strings.Contains(strings.Join(pins["node"], ","), "12") {
		t.Error("pins inside node_modules must be ignored")
	}
}
