package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DUSTIT_CONFIG", filepath.Join(dir, "config.yaml"))
	if err := Load(); err != nil || Current.StaleDays != 30 {
		t.Fatalf("missing file should give defaults: %v %+v", err, Current)
	}
	_ = os.WriteFile(Path(), []byte("stale_days: 60\nexclude: [~/keep]\ndisabled: [trash]\ncategories:\n  - id: x\n    paths: [~/x]\n"), 0o644)
	if err := Load(); err != nil {
		t.Fatal(err)
	}
	if Current.StaleDays != 60 || Current.OldFileDays != 365 || len(Current.Exclude) != 1 || !Current.IsDisabled("trash") {
		t.Fatalf("bad config %+v", Current)
	}
	_ = os.WriteFile(Path(), []byte("categories:\n  - id: y\n"), 0o644)
	if err := Load(); err == nil {
		t.Error("a category without paths must be rejected")
	}
}

func TestExampleParses(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DUSTIT_CONFIG", filepath.Join(dir, "c.yaml"))
	if _, err := Init(); err != nil {
		t.Fatal(err)
	}
	if err := Load(); err != nil {
		t.Fatalf("example config must load: %v", err)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DUSTIT_CONFIG", filepath.Join(dir, "c.yaml"))
	write := func(s string) { _ = os.WriteFile(Path(), []byte(s), 0o644) }
	errs := func(ps []Problem) (n int) {
		for _, p := range ps {
			if p.Error {
				n++
			}
		}
		return
	}

	write("stale_days: [oops\n")
	ps, _ := Check(nil)
	if errs(ps) != 1 || ps[0].Line != 1 {
		t.Errorf("syntax error not reported with its line: %+v", ps)
	}

	write("stale_dayz: 10\nold_file_min: huge\n")
	ps, _ = Check(nil)
	if errs(ps) != 2 || ps[0].Line != 1 || !strings.Contains(ps[0].Msg, `did you mean "stale_days"`) {
		t.Errorf("typo should be reported with a suggestion, and checking should continue: %+v", ps)
	}

	write("stale_days: lots\n")
	if ps, _ = Check(nil); errs(ps) != 1 {
		t.Errorf("wrong type not reported: %+v", ps)
	}

	write("old_file_min: huge\ndisabled: [nope]\ncategories:\n  - id: trash\n    paths: [/etc]\n    risk: maybe\n")
	ps, _ = Check([]string{"trash"})
	if errs(ps) != 4 { // bad size, id clash, path outside home, bad risk
		t.Errorf("want 4 errors, got %+v", ps)
	}
	if len(ps)-errs(ps) != 1 { // unknown disabled id is a warning
		t.Errorf("want 1 warning, got %+v", ps)
	}

	write(Example)
	if ps, err := Check(nil); err != nil || errs(ps) != 0 {
		t.Errorf("the example config must check clean: %v %+v", err, ps)
	}
}
