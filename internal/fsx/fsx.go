// Package fsx measures disk usage and is the only place that removes files.
// Every removal goes through CheckDeletable.
package fsx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Home is the user's home folder. It is set during package initialisation so other
// packages can safely build paths from it in their own package-level variables.
var (
	Home     = mustHome()
	realHome = evalOr(Home)
)

func mustHome() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" || h == "/" {
		fmt.Fprintln(os.Stderr, "cannot determine home directory")
		os.Exit(1)
	}
	return filepath.Clean(h)
}

func evalOr(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// H joins a path relative to the home folder.
func H(rel string) string { return filepath.Join(Home, rel) }

// Expand turns "~/x" or a relative path into an absolute path.
func Expand(p string) string {
	switch {
	case p == "~":
		return Home
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(Home, p[2:])
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// Pretty shortens the home folder to "~".
func Pretty(p string) string {
	if p == Home {
		return "~"
	}
	if strings.HasPrefix(p, Home+"/") {
		return "~/" + p[len(Home)+1:]
	}
	return p
}

// Exists reports whether p exists (without following a final symlink).
func Exists(p string) bool { _, err := os.Lstat(p); return err == nil }

// IsDir reports whether p is a directory (following symlinks).
func IsDir(p string) bool { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

// Usage is the result of measuring a path.
type Usage struct {
	Bytes  int64 // allocated on disk, like du
	Files  int
	Denied int // entries macOS wouldn't let us read
}

// DiskUsage measures a file or folder tree without following symlinks.
func DiskUsage(path string) Usage {
	var u Usage
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			u.Denied++
		}
		return u
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFDIR {
		return Usage{Bytes: st.Blocks * 512, Files: 1}
	}
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				u.Denied++
			}
			if d != nil && d.IsDir() && p != path {
				return fs.SkipDir
			}
			return nil
		}
		var s syscall.Stat_t
		if syscall.Lstat(p, &s) == nil {
			u.Bytes += s.Blocks * 512
			if !d.IsDir() {
				u.Files++
			}
		}
		return nil
	})
	return u
}

// LastTouched is the later of modified and status-changed time. Access time is ignored:
// backups and Spotlight read files constantly, so it says nothing about real use.
func LastTouched(st *syscall.Stat_t) time.Time {
	t := time.Unix(st.Mtimespec.Unix())
	if c := time.Unix(st.Ctimespec.Unix()); c.After(t) {
		t = c
	}
	return t
}

// ModTime is LastTouched for a path; missing paths count as "now".
func ModTime(p string) time.Time {
	var st syscall.Stat_t
	if err := syscall.Lstat(p, &st); err != nil {
		return time.Now()
	}
	return LastTouched(&st)
}

// FreeSpace is the space available to the user on the home volume.
func FreeSpace() int64 {
	var s syscall.Statfs_t
	if err := syscall.Statfs(Home, &s); err != nil {
		return 0
	}
	return int64(s.Bavail) * int64(s.Bsize)
}

// Paths that must never be deleted themselves (their contents may be).
var protectedRel = []string{
	"", "Library", "Library/Caches", "Library/Logs", "Library/Developer",
	"Library/Developer/Xcode", "Library/Developer/Xcode/DerivedData",
	"Library/Application Support", "Library/Containers", "Library/Group Containers",
	"Library/Mobile Documents", "Documents", "Desktop", "Downloads", "Pictures",
	"Movies", "Music", "Applications", ".Trash", ".ssh", ".gnupg",
}

// Areas nothing may ever be deleted from.
var forbiddenRel = []string{
	"Library/Keychains", "Library/Preferences", "Library/Mail", "Library/Messages",
	"Library/Accounts", "Library/Cookies", ".ssh", ".gnupg",
}

// CheckDeletable is the last line of defence before anything is removed.
func CheckDeletable(p string) error {
	if !filepath.IsAbs(p) {
		return fmt.Errorf("refusing relative path %q", p)
	}
	c := filepath.Clean(p)
	rel, err := filepath.Rel(Home, c)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return fmt.Errorf("refusing %s: outside your home folder", c)
	}
	for _, pr := range protectedRel {
		if rel == pr {
			return fmt.Errorf("refusing %s: protected folder", c)
		}
	}
	for _, pr := range forbiddenRel {
		if rel == pr || strings.HasPrefix(rel, pr+"/") {
			return fmt.Errorf("refusing %s: inside protected area", c)
		}
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(c))
	if err != nil {
		return err
	}
	if r, err := filepath.Rel(realHome, parent); err != nil || r == ".." || strings.HasPrefix(r, "../") {
		return fmt.Errorf("refusing %s: resolves outside your home folder", c)
	}
	return nil
}

// Remove deletes p (or moves it to the Trash) after CheckDeletable approves it.
func Remove(p string, toTrash bool) error {
	if err := CheckDeletable(p); err != nil {
		return err
	}
	if toTrash {
		return moveToTrash(p)
	}
	return os.RemoveAll(p) // does not follow symlinks
}

func moveToTrash(p string) error {
	dst := filepath.Join(Home, ".Trash", filepath.Base(p))
	if Exists(dst) {
		ext := filepath.Ext(dst)
		dst = fmt.Sprintf("%s %s%s", strings.TrimSuffix(dst, ext), time.Now().Format("15.04.05.000"), ext)
	}
	return os.Rename(p, dst)
}

// TrashApp moves an .app bundle from an Applications folder to the Trash.
func TrashApp(p string) error {
	ok := false
	for _, base := range []string{"/Applications", H("Applications")} {
		if d := filepath.Dir(p); d == base || filepath.Dir(d) == base {
			ok = true
		}
	}
	if !ok || !strings.HasSuffix(p, ".app") {
		return fmt.Errorf("refusing %s: not an app in an Applications folder", p)
	}
	if fi, err := os.Lstat(p); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing %s: missing or a symlink", p)
	}
	if err := moveToTrash(p); err != nil {
		return fmt.Errorf("%v — the app may need an admin password; drag it to the Trash in Finder", err)
	}
	return nil
}
