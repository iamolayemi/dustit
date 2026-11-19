package finder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/sh"
)

// Group is one plain-language slice of the disk ("Downloads", "App data", …).
type Group struct {
	Name string `json:"name"`
	Hint string `json:"hint,omitempty"` // how to free it
	Size int64  `json:"size_bytes"`
}

// devDotFolders are hidden folders in ~ that belong to developer tools.
var devDotFolders = []string{".npm", ".cargo", ".rustup", "go", ".gradle", ".m2", ".nvm", ".pyenv", ".rbenv",
	".docker", ".minikube", ".cache", ".bun", ".yarn", ".pnpm-store", ".composer", ".android", ".cocoapods",
	".vscode", ".cursor", ".claude", ".codex", ".gemini", ".ollama", ".lmstudio", ".local", ".sdkman", ".volta",
	".asdf", ".pulumi", ".terraform.d", ".kube", "sdk", "fvm", ".dart-tool", ".pub-cache", ".expo"}

// Storage splits the disk into groups. Without Full Disk Access some protected folders
// (other apps' containers, Mail, Messages) can't be measured and fall into "macOS & other".
func Storage() []Group { return StorageFrom(Tree(fsx.Home, 100000)) }

// StorageFrom groups an already-measured home tree (built with a large `keep`, so nothing
// is folded into "other").
func StorageFrom(tree *Node) []Group {
	free, total := fsx.DiskTotals()
	used := total - free

	level1 := map[string]int64{}
	level2 := map[string]int64{}
	var home int64
	for _, n := range tree.Children {
		level1[n.Name] = n.Size
		home += n.Size
		for _, c := range n.Children {
			level2[n.Name+"/"+c.Name] = c.Size
		}
	}

	backups := fsx.DiskUsage(fsx.H("Library/Application Support/MobileSync")).Bytes
	photos := level2["Pictures/Photos Library.photoslibrary"]
	docker, _ := dockerRaw()
	var projects int64
	for _, r := range ProjectRoots() {
		rel := strings.TrimPrefix(r, fsx.Home+"/")
		if !strings.Contains(rel, "/") && rel != "Documents" && rel != "Desktop" {
			projects += level1[rel]
		}
	}
	dev := projects + level2["Library/Developer"] + level2["Library/pnpm"] + docker
	for _, d := range devDotFolders {
		dev += level1[d]
	}
	appData := level2["Library/Application Support"] - backups + level2["Library/Containers"] + level2["Library/Group Containers"] - docker
	apps := fsx.DiskUsage("/Applications").Bytes + level1["Applications"]

	groups := []Group{
		{"Applications", "dustit apps --uninstall", apps},
		{"Documents", "dustit old ~/Documents · dustit dupes", level1["Documents"]},
		{"Desktop", "dustit old ~/Desktop", level1["Desktop"]},
		{"Downloads", "dustit clean --only old-downloads,installers", level1["Downloads"]},
		{"Photos library", "Photos → Settings → iCloud → Optimise Mac Storage", photos},
		{"Pictures (other)", "dustit clean --only screenshots", level1["Pictures"] - photos},
		{"Music", "", level1["Music"]},
		{"Movies", "dustit clean --only large-videos", level1["Movies"]},
		{"iCloud Drive (downloaded)", "dustit clean --only icloud-evict", level2["Library/Mobile Documents"]},
		{"Mail", "Mail → Settings → Accounts → remove old attachments", level2["Library/Mail"]},
		{"Messages", "System Settings → General → Storage → Messages", level2["Library/Messages"]},
		{"iPhone & iPad backups", "dustit clean --only ios-backups", backups},
		{"App data", "dustit storage --apps · dustit leftovers", appData},
		{"Caches & logs", "dustit clean", level2["Library/Caches"] + level2["Library/Logs"]},
		{"Developer files", "dustit clean --only stale-deps,old-versions · dustit docker", dev},
		{"Trash", "dustit clean --only trash", level1[".Trash"]},
	}
	counted := level1["Applications"] // ~/Applications is inside home and already in "Applications"
	for _, g := range groups {
		if g.Name != "Applications" {
			counted += g.Size
		}
	}
	groups = append(groups, Group{"Other files in your home folder", "dustit top", max(home-counted, 0)})

	// Outside the home folder.
	brew := fsx.DiskUsage("/opt/homebrew").Bytes + fsx.DiskUsage("/usr/local").Bytes
	sys := fsx.DiskUsage("/Library").Bytes + fsx.DiskUsage("/Users/Shared").Bytes + fsx.DiskUsage("/private/var").Bytes
	groups = append(groups,
		Group{"Homebrew & command-line tools", "dustit clean --only homebrew,old-versions", brew},
		Group{"System files & temporary data", "/Library, /private/var — mostly managed by macOS", sys},
	)
	data, macos := apfsVolumes()
	if macos > 0 {
		groups = append(groups, Group{"macOS (system, recovery, swap)", "managed by macOS", macos})
	}
	if data == 0 {
		data = used - macos
	}
	rest := data - home - (apps - level1["Applications"]) - brew - sys
	hint := "Purgeable space and folders macOS protects"
	if others := otherUsers(); len(others) > 0 {
		hint = "Other accounts (" + strings.Join(others, ", ") + "), purgeable and protected data"
	}
	groups = append(groups, Group{"Other accounts, protected & purgeable data", hint, max(rest, 0)})
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Size > groups[j].Size })
	return groups
}

// apfsVolumes returns the space used by the Data volume and by macOS's own volumes
// (System, Preboot, Recovery, VM, Update) in the container that holds the home folder.
func apfsVolumes() (data, macos int64) {
	out, err := sh.Quiet("sh", "-c", "diskutil apfs list -plist | plutil -convert json -o - -")
	if err != nil {
		return 0, 0
	}
	var d struct {
		Containers []struct {
			Volumes []struct {
				Roles         []string
				CapacityInUse int64
			}
		}
	}
	if json.Unmarshal([]byte(out), &d) != nil {
		return 0, 0
	}
	for _, c := range d.Containers {
		var dataVol, sys int64
		for _, v := range c.Volumes {
			for _, r := range v.Roles {
				switch r {
				case "Data":
					dataVol += v.CapacityInUse
				case "System", "Preboot", "Recovery", "VM", "Update":
					sys += v.CapacityInUse
				}
			}
		}
		if dataVol > 0 {
			return dataVol, sys
		}
	}
	return 0, 0
}

// otherUsers lists other accounts' home folders in /Users.
func otherUsers() []string {
	entries, _ := os.ReadDir("/Users")
	var out []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() && n != "Shared" && n != "Guest" && !strings.HasPrefix(n, ".") && filepath.Join("/Users", n) != fsx.Home {
			out = append(out, n)
		}
	}
	return out
}

func dockerRaw() (int64, bool) {
	p := fsx.H("Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw")
	if !fsx.MayRead(p) {
		return 0, false
	}
	return fsx.DiskUsage(p).Bytes, true
}

// OwnedFolder is an app-data folder and the app it belongs to.
type OwnedFolder struct {
	Path      string `json:"path"`
	Size      int64  `json:"size_bytes"`
	Owner     string `json:"owner"`
	Installed bool   `json:"installed"`
}

// installedNames maps lower-case bundle ids and app names to the app's display name.
func installedNames() map[string]string {
	names := map[string]string{}
	add := func(id, path string) {
		name := strings.TrimSuffix(filepath.Base(path), ".app")
		if id != "" {
			names[strings.ToLower(id)] = name
		}
		names[strings.ToLower(name)] = name
	}
	const key = "kMDItemCFBundleIdentifier = "
	if out, err := sh.Quiet("mdfind", "-attr", "kMDItemCFBundleIdentifier", "kMDItemContentType == 'com.apple.application-bundle'"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if i := strings.LastIndex(line, key); i >= 0 {
				id := strings.TrimSpace(line[i+len(key):])
				if id == "(null)" {
					id = ""
				}
				add(id, strings.TrimSpace(line[:i]))
			}
		}
	}
	for _, a := range Apps(false) {
		add(a.ID, a.Path)
		if a.BundleName != "" {
			names[strings.ToLower(a.BundleName)] = strings.TrimSuffix(filepath.Base(a.Path), ".app")
		}
	}
	return names
}

// ownerOf names the app a data folder belongs to. It matches bundle ids (also team-prefixed
// group containers like "UBF8T346G9.com.microsoft.word"), the vendor part of a bundle id
// ("com.docker.install" → Docker, "JetBrains" → PhpStorm) and app names ("Antigravity IDE").
func ownerOf(folder string, names map[string]string) (string, bool) {
	f := strings.ToLower(strings.TrimSuffix(folder, ".savedState"))
	if a, ok := ownerAliases[f]; ok {
		f = a
	}
	if n, ok := names[f]; ok {
		return n, true
	}
	if parts := strings.SplitN(f, ".", 2); len(parts) == 2 {
		if n, ok := names[parts[1]]; ok {
			return n, true
		}
	}
	best, bestLen := "", 0
	vendorHits := map[string]bool{}
	for id, n := range names {
		dots := strings.Count(id, ".")
		switch {
		case dots >= 2 && (strings.HasPrefix(f, id+".") || strings.HasSuffix(f, "."+id)) && len(id) > bestLen:
			best, bestLen = n, len(id)
		case dots >= 2 && strings.Count(f, ".") >= 2 && vendor(f) == vendor(id):
			vendorHits[n] = true // same vendor, e.g. com.docker.install → Docker
		case dots >= 1 && !strings.Contains(f, ".") && strings.SplitN(id, ".", 3)[1] == f:
			vendorHits[n] = true // vendor folder, e.g. "jetbrains" → com.jetbrains.*
		case dots == 0 && len(id) >= 4 && (strings.HasPrefix(f, id+" ") || strings.HasPrefix(id, f+" ")):
			vendorHits[n] = true // "antigravity ide" ↔ "antigravity"
		}
	}
	if best != "" {
		return best, true
	}
	if len(vendorHits) > 0 {
		var list []string
		for n := range vendorHits {
			list = append(list, n)
		}
		sort.Strings(list)
		if len(list) > 3 {
			list = append(list[:3], "…")
		}
		return strings.Join(list, ", "), true
	}
	return "", false
}

// AppDataOwners lists the biggest app-data folders and which app owns each.
func AppDataOwners(limit int) []OwnedFolder {
	names := installedNames()
	var all []Item
	for _, d := range []string{"Library/Application Support", "Library/Containers", "Library/Group Containers"} {
		entries, _ := os.ReadDir(fsx.H(d))
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				all = append(all, Item{Path: fsx.H(filepath.Join(d, e.Name()))})
			}
		}
	}
	Measure(all)
	SortBySize(all)
	var out []OwnedFolder
	for _, it := range all {
		if len(out) == limit || it.Size == 0 {
			break
		}
		name := filepath.Base(it.Path)
		owner, ok := ownerOf(name, names)
		if !ok {
			if strings.HasPrefix(strings.ToLower(name), "com.apple.") || strings.Contains(strings.ToLower(name), "apple") {
				owner, ok = "macOS", true
			} else {
				owner = name
			}
		}
		out = append(out, OwnedFolder{Path: it.Path, Size: it.Size, Owner: owner, Installed: ok})
	}
	return out
}
