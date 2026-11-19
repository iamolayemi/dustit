package finder

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/sh"
)

var (
	runningOnce sync.Once
	running     map[string]bool
)

// Folder names that don't match the app's own identifiers.
var ownerAliases = map[string]string{
	"google":         "com.google.chrome",
	"bravesoftware":  "com.brave.browser",
	"microsoft edge": "com.microsoft.edgemac",
	"firefox":        "org.mozilla.firefox",
}

// RunningApps returns lower-case identifiers (bundle id, name, executable, file name) of the
// apps running right now. It's computed once per run.
func RunningApps() map[string]bool {
	runningOnce.Do(func() {
		running = map[string]bool{}
		out, err := sh.Quiet("ps", "-Ao", "comm=")
		if err != nil {
			return
		}
		seen := map[string]bool{}
		var apps []string
		for _, line := range strings.Split(out, "\n") {
			if i := strings.Index(line, ".app/"); i >= 0 {
				app := line[:i+4]
				if !seen[app] {
					seen[app] = true
					apps = append(apps, app)
				}
			}
		}
		var mu sync.Mutex
		Parallel(len(apps), func(i int) {
			keys := []string{strings.TrimSuffix(filepath.Base(apps[i]), ".app")}
			if js, err := sh.Quiet("plutil", "-convert", "json", "-o", "-", filepath.Join(apps[i], "Contents/Info.plist")); err == nil {
				var info map[string]any
				if json.Unmarshal([]byte(js), &info) == nil {
					for _, k := range []string{"CFBundleIdentifier", "CFBundleName", "CFBundleDisplayName", "CFBundleExecutable"} {
						if v, ok := info[k].(string); ok && v != "" {
							keys = append(keys, v)
						}
					}
				}
			}
			mu.Lock()
			for _, k := range keys {
				running[strings.ToLower(k)] = true
			}
			mu.Unlock()
		})
	})
	return running
}

// ownerName is the folder that identifies which app a cache/log/data path belongs to:
// the first component under ~/Library/Caches, Logs or Application Support.
func ownerName(p string) string {
	for _, base := range []string{"Library/Caches", "Library/Logs", "Library/Application Support"} {
		prefix := fsx.H(base) + "/"
		if strings.HasPrefix(p, prefix) {
			return strings.SplitN(strings.TrimPrefix(p, prefix), "/", 2)[0]
		}
	}
	return filepath.Base(p)
}

// BelongsToRunningApp reports whether path p (a cache, log or app-data folder) belongs
// to an app that is open right now.
func BelongsToRunningApp(p string, run map[string]bool) bool {
	n := strings.ToLower(ownerName(p))
	if a, ok := ownerAliases[n]; ok {
		n = a
	}
	if run[n] {
		return true
	}
	for id := range run {
		if strings.Count(id, ".") >= 2 && strings.HasPrefix(n, id+".") {
			return true // e.g. com.tinyspeck.slackmacgap.ShipIt
		}
	}
	return false
}
