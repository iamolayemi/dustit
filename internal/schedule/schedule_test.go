package schedule

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistIsValidAndComplete(t *testing.T) {
	for _, wd := range []int{-1, 1} {
		p, err := Plist(Spec{Binary: "/opt/homebrew/bin/macclean", Categories: []string{"npm-cache", "user-caches"},
			Weekday: wd, Hour: 10, Minute: 30, LogFile: "/tmp/x & y.log"})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"<string>--yes</string>", "<string>npm-cache,user-caches</string>", "<integer>10</integer>", "x &amp; y.log"} {
			if !strings.Contains(p, want) {
				t.Errorf("plist missing %q", want)
			}
		}
		if (wd >= 0) != strings.Contains(p, "<key>Weekday</key>") {
			t.Errorf("weekday %d handled wrongly", wd)
		}
		f := filepath.Join(t.TempDir(), "a.plist")
		_ = os.WriteFile(f, []byte(p), 0o644)
		if out, err := exec.Command("plutil", "-lint", f).CombinedOutput(); err != nil {
			t.Errorf("plutil rejects the plist: %s", out)
		}
	}
}

func TestDefaultsAreSafeOnly(t *testing.T) {
	for _, id := range DefaultCategories {
		if id == "stale-deps" || id == "trash" || id == "electron-caches" {
			t.Errorf("%s must not run unattended by default", id)
		}
	}
}

func TestDescribe(t *testing.T) {
	if got := Describe(1, 9, 5); got != "every Monday at 09:05" {
		t.Errorf("got %q", got)
	}
	if got := Describe(-1, 22, 0); got != "every day at 22:00" {
		t.Errorf("got %q", got)
	}
}
