package targets

import (
	"os"
	"testing"
	"time"

	"github.com/iamolayemi/dustit/internal/history"
)

func TestIDsUniqueAndDescribed(t *testing.T) {
	seen := map[string]bool{}
	for _, tg := range All() {
		if seen[tg.ID] {
			t.Errorf("duplicate id %q", tg.ID)
		}
		seen[tg.ID] = true
		if tg.Name == "" || tg.Desc == "" {
			t.Errorf("%q needs a name and description", tg.ID)
		}
		if len(tg.Dirs) == 0 && tg.Find == nil && len(tg.CleanCmd) == 0 {
			t.Errorf("%q has nothing to clean", tg.ID)
		}
	}
}

func TestSelect(t *testing.T) {
	if _, err := Select([]string{"nope"}, false); err == nil {
		t.Error("unknown ids must be rejected")
	}
	def, _ := Select(nil, false)
	for _, tg := range def {
		if tg.Risk != Safe {
			t.Errorf("default selection must be safe-only, got %q", tg.ID)
		}
	}
	all, _ := Select(nil, true)
	if len(all) != len(All()) {
		t.Error("--all must select every category")
	}
}

func TestDisplayCmd(t *testing.T) {
	got := displayCmd([]string{"sh", "-c", `git -C "$1" worktree prune && git -C "$1" gc --quiet`, "sh", "/r"})
	if got != `git -C "/r" worktree prune && git -C "/r" gc --quiet` {
		t.Errorf("got %q", got)
	}
	if displayCmd([]string{"brew", "uninstall", "x"}) != "brew uninstall x" {
		t.Error("plain commands are joined")
	}
}

func TestRegrowth(t *testing.T) {
	history.Dir = t.TempDir()
	history.Begin("dustit clean")
	history.SetCategory("npm-cache")
	history.Log(history.Deleted, "/a", "", 2e9)
	history.Log(history.Deleted, "/b", "", 1e9)
	// pretend that run happened two days ago
	recs, _ := history.Load()
	_ = recs
	tg := &Target{ID: "npm-cache"}
	res := []Result{{T: tg, Bytes: 2e9}}
	g := Regrowths(res)
	// logged just now: too recent to judge
	if _, ok := g["npm-cache"]; ok {
		t.Error("a clean-up from moments ago shouldn't produce a regrowth estimate")
	}
}

func TestRegrowthDetectsFastRefill(t *testing.T) {
	history.Dir = t.TempDir()
	twoDaysAgo := time.Now().Add(-48 * time.Hour).Format(time.RFC3339Nano)
	line := `{"run":"r1","time":"` + twoDaysAgo + `","command":"dustit clean","category":"npm-cache","action":"deleted","path":"/a","size":2000000000}` + "\n"
	if err := os.MkdirAll(history.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(history.File(), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	res := []Result{{T: &Target{ID: "npm-cache"}, Bytes: 1.6e9}}
	g, ok := Regrowths(res)["npm-cache"]
	if !ok || !g.Fast || g.PerDay < 7e8 || g.PerDay > 9e8 {
		t.Fatalf("1.6 GB back of 2 GB within 2 days should be fast at ~800 MB/day, got %+v", g)
	}
	if d := g.Describe(func(n int64) string { return "X" }); d != "refills ~X/day; cleaned 2 days ago" {
		t.Errorf("describe: %q", d)
	}
}
