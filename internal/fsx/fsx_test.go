package fsx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDeletableRefuses(t *testing.T) {
	for _, p := range []string{
		"/", "/System", "/usr/local", "/Applications/Safari.app", Home, H("Library"),
		H("Library/Caches"), H("Documents"), H("Downloads"), H(".Trash"),
		H("Library/Keychains/login.keychain-db"), H("Library/Preferences/x.plist"),
		H(".ssh/id_ed25519"), H("../other"), "relative/path",
		filepath.Join(t.TempDir(), "x"), // outside home
	} {
		if err := CheckDeletable(p); err == nil {
			t.Errorf("expected %q to be refused", p)
		}
	}
}

func TestCheckDeletableAllows(t *testing.T) {
	for _, p := range []string{H("Library/Caches/com.example.app"), H("Library/Logs/SomeApp")} {
		if !IsDir(filepath.Dir(p)) {
			continue
		}
		if err := CheckDeletable(p); err != nil {
			t.Errorf("expected %q to be allowed: %v", p, err)
		}
	}
}

func TestSymlinkEscapeRefused(t *testing.T) {
	link := H(".macclean-test-link")
	os.Remove(link)
	if err := os.Symlink("/usr", link); err != nil {
		t.Skip(err)
	}
	defer os.Remove(link)
	if err := CheckDeletable(filepath.Join(link, "bin")); err == nil {
		t.Error("a symlinked parent that leaves the home folder must be refused")
	}
}

func TestTrashAppRefusesNonApps(t *testing.T) {
	for _, p := range []string{"/System/Applications/Mail.app", H("Documents/x.app"), "/Applications/notanapp"} {
		if err := TrashApp(p); err == nil {
			t.Errorf("expected %q to be refused", p)
		}
	}
}

func TestExpandPretty(t *testing.T) {
	if got := Expand("~/x"); got != H("x") {
		t.Errorf("Expand(~/x) = %q", got)
	}
	if got := Pretty(H("a/b")); got != "~/a/b" {
		t.Errorf("Pretty = %q", got)
	}
}

func TestExcludes(t *testing.T) {
	SetExcludes([]string{"~/Projects/keep", "~/Downloads/*.dmg"})
	defer SetExcludes(nil)
	for p, want := range map[string]bool{
		H("Projects/keep"):              true,
		H("Projects/keep/node_modules"): true,  // inside
		H("Projects"):                   true,  // contains an excluded path
		H("Projects/other"):             false, // sibling
		H("Downloads/app.dmg"):          true,  // glob
		H("Downloads/app.zip"):          false,
	} {
		if got := Excluded(p); got != want {
			t.Errorf("Excluded(%s) = %v, want %v", Pretty(p), got, want)
		}
	}
	if err := CheckDeletable(H("Projects/keep/node_modules")); err == nil {
		t.Error("excluded paths must be refused")
	}
}

// FuzzCheckDeletable: whatever path it is given, CheckDeletable may only approve paths
// strictly inside the home folder that are not protected or in a forbidden area.
func FuzzCheckDeletable(f *testing.F) {
	for _, s := range []string{"/", Home, H("Library"), H("Library/Caches/x"), H("../x"), H("a/../../b"),
		H("Library/Keychains/k"), H(".ssh"), "relative", H("Documents"), H("Projects/x/node_modules"), H("./Library//Preferences/../Preferences/p")} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		if CheckDeletable(p) != nil {
			return
		}
		c := filepath.Clean(p)
		rel, err := filepath.Rel(Home, c)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			t.Fatalf("approved %q outside home", p)
		}
		for _, pr := range protectedRel {
			if rel == pr {
				t.Fatalf("approved protected %q", p)
			}
		}
		for _, fb := range forbiddenRel {
			if rel == fb || strings.HasPrefix(rel, fb+"/") {
				t.Fatalf("approved forbidden %q", p)
			}
		}
	})
}
