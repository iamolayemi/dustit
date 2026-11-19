// Package targets defines the cleanup categories and how to scan and clean them.
package targets

import (
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/iamolayemi/dustit/internal/config"
	"github.com/iamolayemi/dustit/internal/finder"
	"github.com/iamolayemi/dustit/internal/fsx"
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
	SkipRunning    bool                 // leave items that belong to an app that is open
}

var h = fsx.H

// All returns every enabled category (built-in, then the user's own), safe ones first.
func All() []*Target {
	cfg := config.Current
	var out []*Target
	for _, t := range append(builtin(cfg), custom(cfg)...) {
		if !cfg.IsDisabled(t.ID) {
			out = append(out, t)
		}
	}
	return out
}

// BuiltinIDs lists the ids of the built-in categories.
func BuiltinIDs() []string {
	var ids []string
	for _, t := range builtin(config.Defaults()) {
		ids = append(ids, t.ID)
	}
	return ids
}

// duplicateRoots are the config's duplicate_folders, or personal and project folders.
func duplicateRoots(cfg config.Config) []string {
	if len(cfg.DuplicateFolders) == 0 {
		return finder.UserFileRoots(true)
	}
	var out []string
	for _, f := range cfg.DuplicateFolders {
		out = append(out, fsx.Expand(f))
	}
	return finder.DedupeRoots(out)
}

func size(s string, def int64) int64 {
	if n, err := humanize.ParseBytes(s); err == nil {
		return int64(n)
	}
	return def
}

func custom(cfg config.Config) []*Target {
	var out []*Target
	for _, c := range cfg.Categories {
		t := &Target{ID: c.ID, Name: c.Name, Desc: c.Desc, Risk: Review, ToTrash: c.Trash, MinAgeDays: c.MinAgeDays}
		if c.Risk == "safe" {
			t.Risk = Safe
		}
		if t.Name == "" {
			t.Name = c.ID
		}
		if t.Desc == "" {
			t.Desc = "Custom category from your config"
		}
		for _, p := range c.Paths {
			t.Dirs = append(t.Dirs, fsx.Expand(p))
		}
		out = append(out, t)
	}
	return out
}

func builtin(cfg config.Config) []*Target {
	return []*Target{
		{ID: "user-caches", Name: "App caches", Risk: Safe, SkipRunning: true,
			Desc:    "~/Library/Caches — apps rebuild these automatically; caches of apps that are open are left alone",
			Dirs:    []string{h("Library/Caches")},
			Exclude: []string{"Homebrew", "Yarn", "com.apple.bird", "CloudKit", "com.apple.HomeKit", "FamilyCircle", "com.apple.containermanagerd"}},
		{ID: "user-logs", Name: "Logs & crash reports", Risk: Safe,
			Desc: "~/Library/Logs and crash reporter data",
			Dirs: []string{h("Library/Logs"), h("Library/Application Support/CrashReporter")}},
		{ID: "trash", Name: "Trash", Risk: Safe,
			Desc: "Everything currently in your Trash",
			Dirs: []string{h(".Trash")}},
		{ID: "electron-caches", Name: "Electron & browser app caches", Risk: Safe, SkipRunning: true,
			Desc: "Cache/GPUCache/Code Cache/Service Worker caches of VS Code, Slack, Postman, Figma, Chrome…; apps that are open are left alone",
			Find: finder.ElectronCaches},
		{ID: "dot-cache", Name: "~/.cache tool caches", Risk: Safe,
			Desc:    "XDG caches (puppeteer browsers, copilot, pkg, uv…); AI model caches and corepack's pnpm/yarn are kept",
			Dirs:    []string{h(".cache")},
			Exclude: []string{"huggingface", "torch", "whisper", "ollama", "lm-studio", "gitstatus", "node"}}, // node/corepack holds the pnpm/yarn binaries corepack runs
		{ID: "stale-deps", Name: "Dependencies in idle projects", Risk: Safe,
			Desc: fmt.Sprintf("node_modules, vendor, Pods, target, .venv, .next… in projects untouched for %d+ days (reinstall with npm/composer/pod install)", cfg.StaleDays),
			Find: func() []finder.Item { return finder.StaleDeps(finder.ProjectRoots(), cfg.StaleDays) }},
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
		{ID: "minikube-cache", Name: "minikube image cache", Risk: Safe,
			Desc: "~/.minikube/cache: VM ISOs and preloaded images, downloaded again on the next minikube start",
			Dirs: []string{h(".minikube/cache")}},
		{ID: "cocoapods-specs", Name: "CocoaPods spec repos", Risk: Safe,
			Desc: "~/.cocoapods/repos: cloned spec repos; CocoaPods uses its CDN by default and re-clones if a Podfile asks",
			Dirs: []string{h(".cocoapods/repos")}},
		{ID: "android-cache", Name: "Android build cache", Risk: Safe,
			Desc: "~/.android/cache, rebuilt by Android tools",
			Dirs: []string{h(".android/cache")}},
		{ID: "gradle-daemon-logs", Name: "Gradle daemon logs", Risk: Safe,
			Desc: "Log files left by Gradle daemons in ~/.gradle/daemon",
			Find: finder.GradleDaemonLogs},
		{ID: "editor-orphans", Name: "Editor state for deleted folders", Risk: Safe,
			Desc: "VS Code, Cursor and Windsurf workspace state for folders that no longer exist",
			Find: finder.EditorOrphans},
		{ID: "mail-downloads", Name: "Mail attachment copies", Risk: Safe,
			Desc: "Copies of attachments you opened from Mail (originals stay in Mail)",
			Dirs: []string{h("Library/Containers/com.apple.mail/Data/Library/Mail Downloads")}},

		{ID: "old-files", Name: "Large files untouched for a year", Risk: Review, ToTrash: true,
			Desc: fmt.Sprintf("Files ≥ %s in Desktop, Documents, Movies, Music, Pictures and project folders not changed in %d days — moved to Trash", cfg.OldFileMin, cfg.OldFileDays),
			Find: func() []finder.Item {
				return finder.OldFiles(finder.UserFileRoots(false), cfg.OldFileDays, size(cfg.OldFileMin, 100e6))
			}},
		{ID: "duplicates", Name: "Duplicate files", Risk: Review, ToTrash: true,
			Desc: fmt.Sprintf("Extra copies of identical files ≥ %s in your personal and project folders; the first copy is kept — moved to Trash", cfg.DuplicateMin),
			Find: func() []finder.Item {
				return finder.DupeExtras(finder.Dupes(duplicateRoots(cfg), size(cfg.DuplicateMin, 5e6)))
			}},
		{ID: "installers", Name: "Installers & disk images", Risk: Review, ToTrash: true,
			Desc: ".dmg, .pkg and .xip files older than 7 days in Downloads and on the Desktop — moved to Trash",
			Find: func() []finder.Item { return finder.Installers(7) }},
		{ID: "screenshots", Name: "Old screenshots & recordings", Risk: Review, ToTrash: true,
			Desc: "Screenshots and screen recordings older than 30 days (Desktop, Downloads, Pictures, your screenshot folder) — moved to Trash",
			Find: func() []finder.Item { return finder.Screenshots(30) }},
		{ID: "large-videos", Name: "Large videos untouched for 90 days", Risk: Review, ToTrash: true,
			Desc: "Video files ≥ 500 MB in Movies, Downloads, Desktop and Documents not changed in 90 days — moved to Trash",
			Find: func() []finder.Item { return finder.LargeVideos(90, 500e6) }},
		{ID: "old-downloads", Name: fmt.Sprintf("Downloads older than %d days", cfg.DownloadDays), Risk: Review, ToTrash: true,
			Desc:       "Moved to Trash, not deleted — check before emptying",
			Dirs:       []string{h("Downloads")},
			MinAgeDays: cfg.DownloadDays},
		{ID: "old-versions", Name: "Old language & SDK versions", Risk: Review,
			Desc: "Old Node (nvm/fnm/Volta), Python, Ruby, Rust, Go, Java, Terraform, Flutter, Gradle, Android SDK, asdf/mise tools, " +
				"unused Homebrew python@/php@/… and extra Xcode copies. Keeps your default, the newest, and any version a project pins",
			Find: finder.OldVersions},
		{ID: "ai-models", Name: "Local AI models", Risk: Review,
			Desc: "Ollama, LM Studio and Hugging Face models; re-downloaded when needed",
			Find: finder.AIModels},
		{ID: "macos-installers", Name: "macOS installers", Risk: Review,
			Desc: "\"Install macOS …\" apps in /Applications (moved to Trash); re-downloadable from the App Store",
			Find: finder.MacOSInstallers},
		{ID: "git-gc", Name: "Big git repositories", Risk: Review, UpTo: true,
			Desc: "Repos in your project folders whose .git is over 500 MB: runs `git worktree prune` and `git gc` (history and reflog are kept)",
			Find: func() []finder.Item { return finder.BigGitRepos(finder.ProjectRoots(), 500e6) }},
		{ID: "dev-vms", Name: "Local Kubernetes clusters & VMs", Risk: Review,
			Desc: "minikube, kind, k3d, Colima and Lima clusters/VMs — deleting one removes everything inside it",
			Find: finder.DevVMs},
		{ID: "icloud-evict", Name: "Downloaded iCloud Drive files", Risk: Review,
			Desc: "iCloud Drive files ≥ 10 MB unchanged for 90 days: the local copy is removed, the file stays in iCloud and downloads when opened",
			Find: func() []finder.Item { return finder.ICloudDownloads(90, 10e6) }},
		{ID: "maven-repo", Name: "Maven repository", Risk: Review,
			Desc: "~/.m2/repository: downloaded Java dependencies, fetched again on the next build",
			Dirs: []string{h(".m2/repository")}},
		{ID: "ai-tool-data", Name: "Old AI coding tool sessions", Risk: Review,
			Desc: "Claude Code and Codex session logs older than 60 days and Claude's scratch files older than 30; settings and memory are never touched",
			Find: func() []finder.Item { return finder.AIToolData(60, 30) }},
		{ID: "agent-worktrees", Name: "Idle AI agent worktrees", Risk: Review,
			Desc: "Git worktrees made by AI agents with no uncommitted changes and idle for 14+ days (`git worktree remove`; the branch is kept)",
			Find: func() []finder.Item { return finder.AgentWorktrees(finder.ProjectRoots(), 14) }},
		{ID: "git-branches", Name: "Merged git branches", Risk: Review,
			Desc: "Local branches already merged into the main branch, untouched for 90+ days (`git branch -d` refuses anything unmerged)",
			Find: func() []finder.Item { return finder.MergedBranches(finder.ProjectRoots(), 90) }},
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
			Desc:  "docker system prune -f — for a fuller, guided prune use `dustit docker --clean`",
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
			return nil, fmt.Errorf("unknown category %q — run `dustit list`", id)
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
