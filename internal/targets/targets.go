// Package targets defines the cleanup categories and how to scan and clean them.
package targets

import (
	"fmt"
	"strings"

	"github.com/iamolayemi/macclean/internal/finder"
	"github.com/iamolayemi/macclean/internal/fsx"
)

// Risk says whether a category is cleaned by default.
type Risk int

const (
	// Safe categories are rebuilt automatically and cleaned by default.
	Safe Risk = iota
	// Review categories may hold things you want; they are only cleaned when named.
	Review
)

func (r Risk) String() string {
	if r == Safe {
		return "safe"
	}
	return "review"
}

// Target is one cleanup category. A target removes the *contents* of Dirs, the items
// returned by Find, or runs CleanCmd instead.
type Target struct {
	ID, Name, Desc string
	Risk           Risk
	Dirs           []string             // the contents of these folders are removed
	Exclude        []string             // glob patterns of entry names in Dirs to keep
	MinAgeDays     int                  // only entries older than this
	Find           func() []finder.Item // extra items (size/age filled in if zero)
	ToTrash        bool                 // move to Trash instead of deleting
	Needs          string               // binary that must be installed
	PathCmd        []string             // prints a folder to measure (for CleanCmd targets)
	CleanCmd       []string             // run this instead of deleting
	UpTo           bool                 // measured size is an upper bound
}

var h = fsx.H

// All returns every category, safe ones first.
func All() []*Target {
	return []*Target{
		{ID: "user-caches", Name: "App caches", Risk: Safe,
			Desc:    "~/Library/Caches — apps rebuild these automatically",
			Dirs:    []string{h("Library/Caches")},
			Exclude: []string{"Homebrew", "Yarn", "com.apple.bird", "CloudKit", "com.apple.HomeKit", "FamilyCircle", "com.apple.containermanagerd"}},
		{ID: "user-logs", Name: "Logs & crash reports", Risk: Safe,
			Desc: "~/Library/Logs and crash reporter data",
			Dirs: []string{h("Library/Logs"), h("Library/Application Support/CrashReporter")}},
		{ID: "trash", Name: "Trash", Risk: Safe,
			Desc: "Everything currently in your Trash",
			Dirs: []string{h(".Trash")}},
		{ID: "electron-caches", Name: "Electron & browser app caches", Risk: Safe,
			Desc: "Cache/GPUCache/Code Cache/Service Worker caches of VS Code, Slack, Postman, Figma, Chrome… (quit those apps first)",
			Find: finder.ElectronCaches},
		{ID: "dot-cache", Name: "~/.cache tool caches", Risk: Safe,
			Desc:    "XDG caches (puppeteer browsers, copilot, pkg, uv…); AI model caches and corepack's pnpm/yarn are kept",
			Dirs:    []string{h(".cache")},
			Exclude: []string{"huggingface", "torch", "whisper", "ollama", "lm-studio", "gitstatus", "node"}}, // node/corepack holds the pnpm/yarn binaries corepack runs
		{ID: "stale-deps", Name: "Dependencies in idle projects", Risk: Safe,
			Desc: "node_modules, vendor, Pods, target, .venv, .next… in projects untouched for 30+ days (reinstall with npm/composer/pod install)",
			Find: func() []finder.Item { return finder.StaleDeps(finder.ProjectRoots(), 30) }},
		{ID: "xcode-derived", Name: "Xcode DerivedData", Risk: Safe,
			Desc: "Xcode build intermediates; rebuilt on next build",
			Dirs: []string{h("Library/Developer/Xcode/DerivedData")}},
		{ID: "xcode-device-support", Name: "Xcode device support", Risk: Safe,
			Desc: "Debug symbols for devices; re-downloaded when a device is plugged in",
			Dirs: []string{h("Library/Developer/Xcode/iOS DeviceSupport"), h("Library/Developer/Xcode/watchOS DeviceSupport"),
				h("Library/Developer/Xcode/tvOS DeviceSupport"), h("Library/Developer/Xcode/visionOS DeviceSupport")}},
		{ID: "simulator-caches", Name: "Simulator caches", Risk: Safe,
			Desc: "CoreSimulator caches",
			Dirs: []string{h("Library/Developer/CoreSimulator/Caches")}},
		{ID: "simulators-unavailable", Name: "Unavailable simulators", Risk: Safe,
			Desc:  "Simulators for runtimes no longer installed (xcrun simctl delete unavailable)",
			Needs: "xcrun", CleanCmd: []string{"xcrun", "simctl", "delete", "unavailable"}},
		{ID: "ios-updates", Name: "Old iOS update files", Risk: Safe,
			Desc: "Firmware files downloaded by Finder/iTunes",
			Dirs: []string{h("Library/iTunes/iPhone Software Updates"), h("Library/iTunes/iPad Software Updates")}},
		{ID: "npm-cache", Name: "npm cache", Risk: Safe,
			Desc: "~/.npm/_cacache (same as npm cache clean --force)",
			Dirs: []string{h(".npm/_cacache")}},
		{ID: "yarn-cache", Name: "Yarn cache", Risk: Safe,
			Desc:  "yarn cache clean",
			Needs: "yarn", PathCmd: []string{"yarn", "cache", "dir"}, CleanCmd: []string{"yarn", "cache", "clean"}},
		{ID: "pnpm-store", Name: "pnpm store (unused)", Risk: Safe,
			Desc:  "pnpm store prune — removes packages no project references",
			Needs: "pnpm", UpTo: true, PathCmd: []string{"pnpm", "store", "path"}, CleanCmd: []string{"pnpm", "store", "prune"}},
		{ID: "pnpm-old-stores", Name: "Old pnpm store versions", Risk: Review,
			Desc:  "Stores left by older pnpm versions (e.g. store/v3) that the current pnpm no longer reads",
			Needs: "pnpm", Find: finder.OldPnpmStores},
		{ID: "bun-cache", Name: "Bun cache", Risk: Safe,
			Desc: "~/.bun/install/cache",
			Dirs: []string{h(".bun/install/cache")}},
		{ID: "homebrew", Name: "Homebrew downloads & old versions", Risk: Safe,
			Desc:  "brew cleanup -s --prune=all",
			Needs: "brew", PathCmd: []string{"brew", "--cache"}, CleanCmd: []string{"brew", "cleanup", "-s", "--prune=all"}},
		{ID: "gradle-cache", Name: "Gradle caches", Risk: Safe,
			Desc: "~/.gradle/caches; re-downloaded on next build",
			Dirs: []string{h(".gradle/caches")}},
		{ID: "composer-cache", Name: "Composer cache", Risk: Safe,
			Desc: "PHP Composer download cache",
			Dirs: []string{h(".composer/cache"), h(".cache/composer")}},
		{ID: "cargo-cache", Name: "Cargo registry cache", Risk: Safe,
			Desc: "Downloaded crate sources; re-fetched on next build",
			Dirs: []string{h(".cargo/registry/cache"), h(".cargo/registry/src"), h(".cargo/git/checkouts")}},
		{ID: "mail-downloads", Name: "Mail attachment copies", Risk: Safe,
			Desc: "Copies of attachments you opened from Mail (originals stay in Mail)",
			Dirs: []string{h("Library/Containers/com.apple.mail/Data/Library/Mail Downloads")}},

		{ID: "old-files", Name: "Large files untouched for a year", Risk: Review, ToTrash: true,
			Desc: "Files ≥ 100 MB in Desktop, Documents, Movies, Music, Pictures and project folders not changed in 365 days — moved to Trash",
			Find: func() []finder.Item { return finder.OldFiles(finder.UserFileRoots(false), 365, 100e6) }},
		{ID: "duplicates", Name: "Duplicate files", Risk: Review, ToTrash: true,
			Desc: "Extra copies of identical files ≥ 5 MB in your personal and project folders; the oldest copy is kept — moved to Trash",
			Find: func() []finder.Item { return finder.DupeExtras(finder.Dupes(finder.UserFileRoots(true), 5e6)) }},
		{ID: "old-downloads", Name: "Downloads older than 90 days", Risk: Review, ToTrash: true,
			Desc:       "Moved to Trash, not deleted — check before emptying",
			Dirs:       []string{h("Downloads")},
			MinAgeDays: 90},
		{ID: "jetbrains-old", Name: "Old JetBrains IDE versions", Risk: Review,
			Desc: "Settings/plugins of IDE versions older than the newest one you have, plus -backup folders",
			Find: finder.JetBrainsOld},
		{ID: "iac-plugins", Name: "sst / Pulumi / Terraform plugins", Risk: Review,
			Desc: "Downloaded provider plugins; re-downloaded on next deploy",
			Dirs: []string{h("Library/Application Support/sst/plugins"), h(".pulumi/plugins"), h(".terraform.d/plugin-cache")}},
		{ID: "go-modcache", Name: "Go module cache", Risk: Review,
			Desc:  "go clean -modcache; modules re-download on next build",
			Needs: "go", PathCmd: []string{"go", "env", "GOMODCACHE"}, CleanCmd: []string{"go", "clean", "-modcache"}},
		{ID: "docker", Name: "Docker unused data", Risk: Review,
			Desc:  "docker system prune -f — for a fuller, guided prune use `macclean docker --clean`",
			Needs: "docker", CleanCmd: []string{"docker", "system", "prune", "-f"}},
		{ID: "xcode-archives", Name: "Xcode archives", Risk: Review,
			Desc: "App builds you archived; needed to symbolicate old crash logs",
			Dirs: []string{h("Library/Developer/Xcode/Archives")}},
		{ID: "simulator-devices", Name: "iOS simulator devices", Risk: Review,
			Desc:    "All simulator devices and their app data",
			Dirs:    []string{h("Library/Developer/CoreSimulator/Devices")},
			Exclude: []string{"device_set.plist"}},
		{ID: "ios-backups", Name: "iPhone/iPad backups", Risk: Review,
			Desc: "Local device backups — deleted permanently",
			Dirs: []string{h("Library/Application Support/MobileSync/Backup")}},
	}
}

// Select returns the targets named in ids, or the default set (safe, plus review if all).
func Select(ids []string, all bool) ([]*Target, error) {
	ts := All()
	if len(ids) == 0 {
		var out []*Target
		for _, t := range ts {
			if t.Risk == Safe || all {
				out = append(out, t)
			}
		}
		return out, nil
	}
	byID := map[string]*Target{}
	for _, t := range ts {
		byID[t.ID] = t
	}
	var out []*Target
	for _, id := range ids {
		t, ok := byID[strings.TrimSpace(id)]
		if !ok {
			return nil, fmt.Errorf("unknown category %q — run `macclean list`", id)
		}
		out = append(out, t)
	}
	return out, nil
}

// IDs lists every category id (for shell completion).
func IDs() []string {
	var out []string
	for _, t := range All() {
		out = append(out, t.ID+"\t"+t.Name)
	}
	return out
}
