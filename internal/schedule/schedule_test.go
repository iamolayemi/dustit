package schedule

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlistIsValidAndComplete(t *testing.T) {
	for _, wd := range []int{-1, 1} {
		p, err := Plist(Spec{Binary: "/opt/homebrew/bin/dustit", Categories: []string{"npm-cache", "user-caches"},
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

func TestAlertPlistAndThreshold(t *testing.T) {
	p, err := AlertPlist(AlertSpec{Binary: "/x/dustit", Below: "10%", Every: time.Hour})
	if err != nil || !strings.Contains(p, "<integer>3600</integer>") || !strings.Contains(p, "<string>alert</string>") {
		t.Fatalf("bad alert plist: %v\n%s", err, p)
	}
	f := filepath.Join(t.TempDir(), "a.plist")
	_ = os.WriteFile(f, []byte(p), 0o644)
	if out, err := exec.Command("plutil", "-lint", f).CombinedOutput(); err != nil {
		t.Errorf("plutil rejects the alert plist: %s", out)
	}
	size := func(s string) (int64, error) { return map[string]int64{"20GB": 20e9}[s], nil }
	pct, _ := Threshold("10%", size)
	if !pct(9, 100) || pct(11, 100) {
		t.Error("percentage threshold wrong")
	}
	abs, _ := Threshold("20GB", size)
	if !abs(19e9, 500e9) || abs(21e9, 500e9) {
		t.Error("size threshold wrong")
	}
	if _, err := Threshold("150%", size); err == nil {
		t.Error("150% must be rejected")
	}
}
