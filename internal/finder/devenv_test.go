package finder

import (
	"os/exec"
	"path/filepath"
	"testing"
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
