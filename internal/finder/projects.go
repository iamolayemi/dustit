package finder

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/iamolayemi/macclean/internal/fsx"
)

func humanAgo(t time.Time) string { return humanize.Time(t) }

// artifactRules: dependency/build folder name → files that must sit next to it
// (a trailing "@" means the file must be inside the folder instead).
var artifactRules = map[string][]string{
	"node_modules":  {"package.json"},
	".next":         {"package.json"},
	".nuxt":         {"package.json"},
	".turbo":        {"package.json"},
	".svelte-kit":   {"package.json"},
	".parcel-cache": {"package.json"},
	".angular":      {"angular.json"},
	"vendor":        {"composer.json"},
	"Pods":          {"Podfile"},
	"target":        {"Cargo.toml", "pom.xml"},
	".gradle":       {"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"},
	"build":         {"build.gradle", "build.gradle.kts", "pubspec.yaml"},
	".dart_tool":    {"pubspec.yaml"},
	".venv":         {"pyvenv.cfg@"},
	"venv":          {"pyvenv.cfg@"},
}

func isArtifact(name string) bool { _, ok := artifactRules[name]; return ok }

func hasMarker(parent, name string, markers []string) bool {
	for _, m := range markers {
		p := filepath.Join(parent, m)
		if strings.HasSuffix(m, "@") {
			p = filepath.Join(parent, name, strings.TrimSuffix(m, "@"))
		}
		if fsx.Exists(p) {
			return true
		}
	}
	return false
}

// lastActivity is the newest change in a project, ignoring dependency folders. It stops early
// once it finds anything newer than cutoff.
func lastActivity(dir string, cutoff time.Time) time.Time {
	var latest time.Time
	for _, g := range []string{".git/index", ".git/HEAD", ".git/FETCH_HEAD"} {
		if t := fsx.ModTime(filepath.Join(dir, g)); fsx.Exists(filepath.Join(dir, g)) && t.After(latest) {
			latest = t
		}
	}
	if latest.After(cutoff) {
		return latest
	}
	count := 0
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && p != dir && (isArtifact(d.Name()) || d.Name() == ".git") {
			return fs.SkipDir
		}
		if fi, err := d.Info(); err == nil && fi.ModTime().After(latest) {
			latest = fi.ModTime()
		}
		count++
		if latest.After(cutoff) || count > 50000 {
			return fs.SkipAll
		}
		return nil
	})
	return latest
}

// StaleDeps finds dependency/build folders in projects untouched for `days` days (0 = all).
func StaleDeps(roots []string, days int) []Item {
	var found []string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() && p != root {
					return fs.SkipDir
				}
				return nil
			}
			if !d.IsDir() || p == root {
				return nil
			}
			name := d.Name()
			if p == fsx.H("Library") || name == ".git" || name == ".Trash" || strings.HasSuffix(name, ".app") {
				return fs.SkipDir
			}
			if markers, ok := artifactRules[name]; ok && hasMarker(filepath.Dir(p), name, markers) {
				found = append(found, p)
				return fs.SkipDir
			}
			if strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		})
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	activity := map[string]time.Time{}
	var items []Item
	for _, p := range found {
		proj := filepath.Dir(p)
		act, ok := activity[proj]
		if !ok {
			act = lastActivity(proj, cutoff)
			activity[proj] = act
		}
		if days > 0 && act.After(cutoff) {
			continue
		}
		items = append(items, Item{Path: p, Age: act, Note: "project idle " + humanize.RelTime(act, time.Now(), "", "")})
	}
	items = withoutExcluded(items)
	Measure(items)
	SortBySize(items)
	return items
}
