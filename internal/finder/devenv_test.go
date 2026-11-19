package finder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/iamolayemi/macclean/internal/fsx"
)

func TestBigGitRepos(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	for _, repo := range []string{"big", "small", "big/node_modules/dep"} {
		dir := filepath.Join(root, repo)
		writeFile(t, filepath.Join(dir, "f.bin"), make([]byte, 1), 0)
		if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
			t.Fatal(string(out))
		}
	}
	writeFile(t, filepath.Join(root, "big/.git/objects/pack/fake.pack"), make([]byte, 2<<20), 0)
	got := BigGitRepos([]string{root}, 1<<20)
	if len(got) != 1 || got[0].Path != filepath.Join(root, "big/.git") {
		t.Fatalf("want only big/.git, got %+v", got)
	}
	if len(got[0].Cmd) == 0 || got[0].Cmd[len(got[0].Cmd)-1] != filepath.Join(root, "big") {
		t.Errorf("gc must run on the repo: %v", got[0].Cmd)
	}
}

func TestBelongsToRunningApp(t *testing.T) {
	run := map[string]bool{"com.tinyspeck.slackmacgap": true, "code": true, "com.google.chrome": true}
	for p, want := range map[string]bool{
		fsx.H("Library/Caches/com.tinyspeck.slackmacgap"):          true,
		fsx.H("Library/Caches/com.tinyspeck.slackmacgap.ShipIt"):   true,
		fsx.H("Library/Application Support/Code/Cache"):            true,
		fsx.H("Library/Application Support/Google/Chrome/Default"): true,
		fsx.H("Library/Caches/com.spotify.client"):                 false,
		fsx.H("Library/Application Support/Postman/Cache"):         false,
	} {
		if got := BelongsToRunningApp(p, run); got != want {
			t.Errorf("%s: got %v, want %v", fsx.Pretty(p), got, want)
		}
	}
}

func TestOwnerOf(t *testing.T) {
	names := map[string]string{"com.tinyspeck.slackmacgap": "Slack", "slack": "Slack", "com.microsoft.word": "Microsoft Word",
		"code": "Visual Studio Code", "com.docker.docker": "Docker", "com.jetbrains.phpstorm": "PhpStorm", "antigravity": "Antigravity"}
	for folder, want := range map[string]string{
		"com.tinyspeck.slackmacgap":        "Slack",
		"UBF8T346G9.com.microsoft.word":    "Microsoft Word",
		"group.com.tinyspeck.slackmacgap":  "Slack",
		"com.tinyspeck.slackmacgap.ShipIt": "Slack",
		"Code":                             "Visual Studio Code",
		"com.docker.install":               "Docker",
		"JetBrains":                        "PhpStorm",
		"Antigravity IDE":                  "Antigravity",
	} {
		if got, ok := ownerOf(folder, names); !ok || got != want {
			t.Errorf("ownerOf(%q) = %q, %v; want %q", folder, got, ok, want)
		}
	}
	if _, ok := ownerOf("com.spotify.client", names); ok {
		t.Error("an app that isn't installed must not get an owner")
	}
}

func TestMergedBranches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "app")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2020-01-01T00:00:00", "GIT_COMMITTER_DATE=2020-01-01T00:00:00",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	_ = os.MkdirAll(repo, 0o755)
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "init")
	run("branch", "feature-done")      // merged (same commit), old
	run("checkout", "-q", "-b", "wip") // not merged
	run("commit", "-q", "--allow-empty", "-m", "wip")
	run("checkout", "-q", "main")

	items := MergedBranches([]string{root}, 90)
	if len(items) != 1 {
		t.Fatalf("want one repo, got %+v", items)
	}
	cmd := items[0].Cmd
	if cmd[len(cmd)-1] != "feature-done" || len(cmd) != 6 {
		t.Errorf("only the merged branch should be deleted, got %v", cmd)
	}
}
