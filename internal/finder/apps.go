package finder

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/sh"
)

// App is an installed application.
type App struct {
	Name, Path, ID string
	BundleName     string // CFBundleName, e.g. "Code" for Visual Studio Code
	Size           int64
	LastUsed       time.Time // zero = never opened, as far as Spotlight knows
}

func bundleID(app string) string {
	return sh.Line("plutil", "-extract", "CFBundleIdentifier", "raw", "-o", "-", filepath.Join(app, "Contents/Info.plist"))
}

func lastUsed(app string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05 -0700", sh.Line("mdls", "-raw", "-name", "kMDItemLastUsedDate", app))
	if err != nil {
		return time.Time{}
	}
	return t
}

// Apps lists .app bundles in /Applications and ~/Applications (one sub-folder level deep).
func Apps(withSize bool) []App {
	var paths []string
	for _, d := range []string{"/Applications", fsx.H("Applications")} {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			p := filepath.Join(d, e.Name())
			if strings.HasSuffix(e.Name(), ".app") {
				paths = append(paths, p)
				continue
			}
			if e.IsDir() {
				sub, _ := os.ReadDir(p)
				for _, s := range sub {
					if strings.HasSuffix(s.Name(), ".app") {
						paths = append(paths, filepath.Join(p, s.Name()))
					}
				}
			}
		}
	}
	apps := make([]App, len(paths))
	Parallel(len(paths), func(i int) {
		p := paths[i]
		a := App{Name: strings.TrimSuffix(filepath.Base(p), ".app"), Path: p, ID: bundleID(p), LastUsed: lastUsed(p),
			BundleName: sh.Line("plutil", "-extract", "CFBundleName", "raw", "-o", "-", filepath.Join(p, "Contents/Info.plist"))}
		if withSize {
			a.Size = fsx.DiskUsage(p).Bytes
		}
		apps[i] = a
	})
	return apps
}

// AppData returns the per-user data folders that belong to an app.
func AppData(a App) []string {
	var c []string
	if a.ID != "" {
		for _, rel := range []string{"Containers/", "Caches/", "Application Support/", "HTTPStorages/", "WebKit/", "Logs/"} {
			c = append(c, fsx.H("Library/"+rel+a.ID))
		}
		c = append(c, fsx.H("Library/Saved Application State/"+a.ID+".savedState"),
			fsx.H("Library/HTTPStorages/"+a.ID+".binarycookies"))
	}
	c = append(c, fsx.H("Library/Application Support/"+a.Name), fsx.H("Library/Caches/"+a.Name), fsx.H("Library/Logs/"+a.Name))
	var out []string
	for _, p := range c {
		if fsx.Exists(p) {
			out = append(out, p)
		}
	}
	return out
}

var reverseDNS = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9_-]+){2,}$`)

// InstalledIDs collects bundle IDs of every app Spotlight or the Applications folders know about.
func InstalledIDs() map[string]bool {
	ids := map[string]bool{}
	const key = "kMDItemCFBundleIdentifier = "
	if out, err := sh.Quiet("mdfind", "-attr", "kMDItemCFBundleIdentifier", "kMDItemContentType == 'com.apple.application-bundle'"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if i := strings.LastIndex(line, key); i >= 0 {
				if id := strings.TrimSpace(line[i+len(key):]); id != "" && id != "(null)" {
					ids[strings.ToLower(id)] = true
				}
			}
		}
	}
	for _, a := range Apps(false) {
		if a.ID != "" {
			ids[strings.ToLower(a.ID)] = true
		}
	}
	return ids
}

func vendor(id string) string {
	parts := strings.Split(id, ".")
	if len(parts) < 2 {
		return id
	}
	return parts[0] + "." + parts[1]
}

// BelongsToInstalled is deliberately generous: anything from the same vendor counts as installed.
func BelongsToInstalled(id string, ids map[string]bool) bool {
	id = strings.ToLower(id)
	if ids[id] {
		return true
	}
	v := vendor(id)
	for x := range ids {
		if strings.HasPrefix(id, x+".") || strings.HasPrefix(x, id+".") || vendor(x) == v {
			return true
		}
	}
	return false
}

// Leftovers finds data folders named after bundle IDs of apps that are no longer installed.
func Leftovers() ([]Item, error) {
	ids := InstalledIDs()
	if len(ids) < 5 {
		return nil, errors.New("could only find " + strconv.Itoa(len(ids)) + " installed apps — Spotlight may be off; refusing to guess")
	}
	places := []string{"Library/Containers", "Library/Caches", "Library/Application Support",
		"Library/Saved Application State", "Library/HTTPStorages", "Library/WebKit", "Library/Logs"}
	var found []Item
	for _, pl := range places {
		entries, _ := os.ReadDir(fsx.H(pl))
		for _, e := range entries {
			id := strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".savedState"), ".binarycookies")
			low := strings.ToLower(id)
			if !reverseDNS.MatchString(id) || strings.HasPrefix(low, "com.apple.") ||
				strings.HasPrefix(low, "group.") || strings.HasPrefix(low, "systemgroup.") ||
				BelongsToInstalled(id, ids) {
				continue
			}
			p := filepath.Join(fsx.H(pl), e.Name())
			found = append(found, Item{Path: p, Age: fsx.ModTime(p)})
		}
	}
	found = withoutExcluded(found)
	Measure(found)
	SortBySize(found)
	return found, nil
}
