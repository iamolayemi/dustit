package finder

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/sh"
)

// versionTool describes where a version manager keeps installed versions and how to
// tell which ones are in use.
type versionTool struct {
	Tool       string   // key used for project pins ("node", "python", …)
	Manager    string   // shown to the user
	Dirs       []string // folders (globs, relative to ~) whose children are versions
	Default    func() []string
	KeepNames  *regexp.Regexp // names always kept (e.g. rustup channels)
	KeepNewest int            // newest N kept (default 1; 0 = none)
	PerMajor   bool           // keep the newest of each major version instead
}

func readFirstLine(rel string) []string {
	b, err := os.ReadFile(fsx.H(rel))
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

func linkTarget(rel string) []string {
	t, err := os.Readlink(fsx.H(rel))
	if err != nil {
		return nil
	}
	return []string{filepath.Base(t)}
}

// nvmDefault follows nvm's alias chain (default → lts/* → lts/krypton → v24.18.0).
func nvmDefault() []string {
	name := "default"
	for i := 0; i < 6; i++ {
		v := readFirstLine(".nvm/alias/" + name)
		if len(v) == 0 {
			return []string{name}
		}
		name = v[0]
		if versionNums(name) != nil {
			return []string{name}
		}
	}
	return nil
}

var volta = regexp.MustCompile(`"node"\s*:\s*\{[^}]*"runtime"\s*:\s*"([^"]+)"`)

func voltaDefault() []string {
	b, _ := os.ReadFile(fsx.H(".volta/tools/user/platform.json"))
	if m := volta.FindSubmatch(b); m != nil {
		return []string{string(m[1])}
	}
	return nil
}

func rustupDefault() []string {
	for _, l := range readFirstLine(".rustup/settings.toml") {
		if k, v, ok := strings.Cut(l, "="); ok && strings.TrimSpace(k) == "default_toolchain" {
			return []string{strings.Trim(strings.TrimSpace(v), `"`)}
		}
	}
	return nil
}

func goDefault() []string {
	// "go version go1.22.3 darwin/arm64"
	if f := strings.Fields(sh.Line("go", "version")); len(f) > 2 {
		return []string{f[2]}
	}
	return nil
}

func toolVersionsDefault(tool string) func() []string {
	return func() []string { return parseToolVersions(fsx.H(".tool-versions"))[tool] }
}

var versionTools = []versionTool{
	{Tool: "node", Manager: "nvm", Dirs: []string{".nvm/versions/node"}, Default: nvmDefault},
	{Tool: "node", Manager: "fnm", Dirs: []string{".local/share/fnm/node-versions", "Library/Application Support/fnm/node-versions"},
		Default: func() []string { return linkTarget(".local/share/fnm/aliases/default") }},
	{Tool: "node", Manager: "Volta", Dirs: []string{".volta/tools/image/node"}, Default: voltaDefault},
	{Tool: "python", Manager: "pyenv", Dirs: []string{".pyenv/versions"}, Default: func() []string { return readFirstLine(".pyenv/version") }},
	{Tool: "ruby", Manager: "rbenv", Dirs: []string{".rbenv/versions"}, Default: func() []string { return readFirstLine(".rbenv/version") }},
	{Tool: "ruby", Manager: "rvm", Dirs: []string{".rvm/rubies"}, Default: func() []string { return linkTarget(".rvm/rubies/default") }},
	{Tool: "rust", Manager: "rustup", Dirs: []string{".rustup/toolchains"}, Default: rustupDefault,
		KeepNames: regexp.MustCompile(`^(stable|beta|nightly)-[a-z0-9_]+-apple-darwin$`), KeepNewest: -1},
	{Tool: "go", Manager: "Go SDKs (golang.org/dl)", Dirs: []string{"sdk"}, Default: goDefault},
	{Tool: "go", Manager: "goenv", Dirs: []string{".goenv/versions"}, Default: func() []string { return readFirstLine(".goenv/version") }},
	{Tool: "terraform", Manager: "tfenv", Dirs: []string{".tfenv/versions"}, Default: func() []string { return readFirstLine(".tfenv/version") }},
	{Tool: "java", Manager: "JDKs", Dirs: []string{"Library/Java/JavaVirtualMachines"}, PerMajor: true},
	{Tool: "java", Manager: "SDKMAN java", Dirs: []string{".sdkman/candidates/java"}, Default: func() []string { return linkTarget(".sdkman/candidates/java/current") }, PerMajor: true},
	{Tool: "gradle", Manager: "SDKMAN gradle", Dirs: []string{".sdkman/candidates/gradle"}, Default: func() []string { return linkTarget(".sdkman/candidates/gradle/current") }},
	{Tool: "gradle", Manager: "Gradle wrapper", Dirs: []string{".gradle/wrapper/dists"}, KeepNewest: 2},
	{Tool: "flutter", Manager: "fvm", Dirs: []string{"fvm/versions"}, Default: func() []string { return linkTarget("fvm/default") }},
	{Tool: "android-build-tools", Manager: "Android build-tools", Dirs: []string{"Library/Android/sdk/build-tools"}},
	{Tool: "android-platform", Manager: "Android platforms", Dirs: []string{"Library/Android/sdk/platforms"}, KeepNewest: 2},
	{Tool: "android-ndk", Manager: "Android NDK", Dirs: []string{"Library/Android/sdk/ndk"}},
	{Tool: "android-image", Manager: "Android system images", Dirs: []string{"Library/Android/sdk/system-images"}, KeepNewest: 2},
}

// asdf and mise keep one folder per tool; each becomes its own versionTool.
func pluginTools() []versionTool {
	var out []versionTool
	for _, m := range []struct{ manager, dir string }{{"asdf", ".asdf/installs"}, {"mise", ".local/share/mise/installs"}} {
		entries, _ := os.ReadDir(fsx.H(m.dir))
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			tool := normalizeTool(e.Name())
			def := toolVersionsDefault(tool)
			if m.manager == "mise" {
				def = func() []string {
					return append(miseConfig(fsx.H(".config/mise/config.toml"))[tool], toolVersionsDefault(tool)()...)
				}
			}
			out = append(out, versionTool{Tool: tool, Manager: m.manager + " " + e.Name(), Dirs: []string{filepath.Join(m.dir, e.Name())}, Default: def})
		}
	}
	return out
}

func normalizeTool(name string) string {
	switch name {
	case "nodejs":
		return "node"
	case "golang":
		return "go"
	}
	return name
}

// ---------- version strings ----------

var numsRe = regexp.MustCompile(`\d+(?:\.\d+)*`)

// versionNums extracts the numeric version from a folder name or pin
// ("v22.14.0" → [22 14 0], "gradle-8.5-bin" → [8 5], "openjdk-17.0.2" → [17 0 2]).
func versionNums(s string) []int {
	m := numsRe.FindString(s)
	if m == "" {
		return nil
	}
	var out []int
	for _, p := range strings.Split(m, ".") {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}

func compareNums(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return len(a) - len(b)
}

// matchesPin reports whether an installed version satisfies a pin: an exact name, or a
// numeric prefix ("22" matches 22.14.0, "3.10" matches 3.10.13).
func matchesPin(name, pin string) bool {
	pin = strings.TrimSpace(pin)
	if pin == "" {
		return false
	}
	if name == pin || strings.TrimPrefix(name, "v") == strings.TrimPrefix(pin, "v") {
		return true
	}
	pn, vn := versionNums(strings.TrimLeft(pin, "^~>=<v ")), versionNums(name)
	if pn == nil || vn == nil || len(pn) > len(vn) {
		return false
	}
	for i := range pn {
		if pn[i] != vn[i] {
			return false
		}
	}
	return true
}

// ---------- project pins ----------

var (
	tomlKV      = regexp.MustCompile(`^\s*"?([A-Za-z0-9_-]+)"?\s*=\s*"?([^"\s#]+)"?`)
	gradleDist  = regexp.MustCompile(`gradle-(\d[\d.]*)-`)
	voltaPin    = regexp.MustCompile(`"volta"\s*:\s*\{[^}]*"node"\s*:\s*"([^"]+)"`)
	fvmPin      = regexp.MustCompile(`"(?:flutter|flutterSdkVersion)"\s*:\s*"([^"]+)"`)
	channelLine = regexp.MustCompile(`channel\s*=\s*"([^"]+)"`)
	composerPHP = regexp.MustCompile(`"php"\s*:\s*"[^\d"]*(\d+\.\d+)`)
)

func parseToolVersions(path string) map[string][]string {
	out := map[string][]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(strings.SplitN(l, "#", 2)[0])
		if len(f) >= 2 {
			t := normalizeTool(f[0])
			out[t] = append(out[t], f[1:]...)
		}
	}
	return out
}

func miseConfig(path string) map[string][]string {
	out := map[string][]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	inTools := false
	for _, l := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			inTools = t == "[tools]"
			continue
		}
		if m := tomlKV.FindStringSubmatch(t); inTools && m != nil {
			tool := normalizeTool(m[1])
			out[tool] = append(out[tool], m[2])
		}
	}
	return out
}

func firstLines(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

// pinFiles maps a project file name to a parser returning tool → versions.
var pinFiles = map[string]func(path string) map[string][]string{
	".nvmrc":             single("node"),
	".node-version":      single("node"),
	".python-version":    single("python"),
	".ruby-version":      single("ruby"),
	".go-version":        single("go"),
	".terraform-version": single("terraform"),
	".java-version":      single("java"),
	"rust-toolchain":     single("rust"),
	".tool-versions":     parseToolVersions,
	"mise.toml":          miseConfig,
	".mise.toml":         miseConfig,
	"rust-toolchain.toml": func(p string) map[string][]string {
		b, _ := os.ReadFile(p)
		if m := channelLine.FindSubmatch(b); m != nil {
			return map[string][]string{"rust": {string(m[1])}}
		}
		return nil
	},
	".sdkmanrc": func(p string) map[string][]string {
		out := map[string][]string{}
		for _, l := range firstLines(p) {
			if k, v, ok := strings.Cut(l, "="); ok {
				out[strings.TrimSpace(k)] = append(out[strings.TrimSpace(k)], strings.TrimSpace(v))
			}
		}
		return out
	},
	"package.json":              regexPin("node", voltaPin),
	"composer.json":             regexPin("php", composerPHP),
	".fvmrc":                    regexPin("flutter", fvmPin),
	"fvm_config.json":           regexPin("flutter", fvmPin),
	"gradle-wrapper.properties": regexPin("gradle", gradleDist),
}

func single(tool string) func(string) map[string][]string {
	return func(p string) map[string][]string {
		return map[string][]string{tool: firstLines(p)}
	}
}

func regexPin(tool string, re *regexp.Regexp) func(string) map[string][]string {
	return func(p string) map[string][]string {
		b, _ := os.ReadFile(p)
		if m := re.FindSubmatch(b); m != nil {
			return map[string][]string{tool: {string(m[1])}}
		}
		return nil
	}
}

// ProjectPins finds version pins in project folders (up to 4 levels deep).
func ProjectPins(roots []string) map[string][]string {
	pins := map[string][]string{}
	for _, root := range roots {
		depth0 := strings.Count(root, "/")
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() && p != root {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if p != root && (strings.Count(p, "/")-depth0 > 6 || isArtifact(name) || name == ".git" ||
					(strings.HasPrefix(name, ".") && name != ".fvm") || p == fsx.H("Library")) {
					return fs.SkipDir
				}
				return nil
			}
			if parse, ok := pinFiles[d.Name()]; ok && strings.Count(p, "/")-depth0 <= 6 {
				for tool, vs := range parse(p) {
					pins[tool] = append(pins[tool], vs...)
				}
			}
			return nil
		})
	}
	return pins
}

// ---------- finding old versions ----------

type installed struct {
	path, name string
	nums       []int
}

func (t versionTool) old(pins map[string][]string) []Item {
	var vs []installed
	for _, d := range t.Dirs {
		if !filepath.IsAbs(d) {
			d = fsx.H(d)
		}
		dirs, _ := filepath.Glob(d)
		for _, dir := range dirs {
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if !e.IsDir() || e.Type()&os.ModeSymlink != 0 || strings.HasPrefix(e.Name(), ".") ||
					e.Name() == "current" || e.Name() == "default" {
					continue
				}
				vs = append(vs, installed{filepath.Join(dir, e.Name()), e.Name(), versionNums(e.Name())})
			}
		}
	}
	if len(vs) < 2 {
		return nil
	}

	keep := map[string]string{} // path → reason
	var defaults []string
	if t.Default != nil {
		defaults = t.Default()
	}
	for _, v := range vs {
		for _, d := range defaults {
			if matchesPin(v.name, d) {
				keep[v.path] = "default"
			}
		}
		for _, p := range pins[t.Tool] {
			if _, ok := keep[v.path]; !ok && matchesPin(v.name, p) {
				keep[v.path] = "used by a project"
			}
		}
		if t.KeepNames != nil && t.KeepNames.MatchString(v.name) {
			keep[v.path] = "channel"
		}
	}

	sort.Slice(vs, func(i, j int) bool { return compareNums(vs[i].nums, vs[j].nums) > 0 })
	n := t.KeepNewest
	if n == 0 {
		n = 1
	}
	if t.PerMajor {
		seen := map[int]bool{}
		for _, v := range vs {
			if len(v.nums) > 0 && !seen[v.nums[0]] {
				seen[v.nums[0]] = true
				if _, ok := keep[v.path]; !ok {
					keep[v.path] = "newest of its major"
				}
			}
		}
	} else {
		for i := 0; i < n && i < len(vs); i++ {
			if _, ok := keep[vs[i].path]; !ok && vs[i].nums != nil {
				keep[vs[i].path] = "newest"
			}
		}
	}

	var kept []string
	for _, v := range vs {
		if r, ok := keep[v.path]; ok {
			kept = append(kept, v.name+" ("+r+")")
		}
	}
	var out []Item
	for _, v := range vs {
		if _, ok := keep[v.path]; ok || v.nums == nil {
			continue // never remove something we can't read a version from
		}
		out = append(out, Item{Path: v.path, Note: t.Manager + " — keeping " + strings.Join(kept, ", ")})
	}
	return out
}

// ---------- Homebrew versioned formulae ----------

var atVersion = regexp.MustCompile(`^(.+)@([\d.]+)$`)

// OldBrewVersions finds versioned formulae (python@3.11, php@8.1…) when a newer version of
// the same formula is installed, nothing installed depends on them, and they aren't running.
func OldBrewVersions(pins map[string][]string) []Item {
	if !sh.Has("brew") {
		return nil
	}
	out, err := sh.Quiet("brew", "list", "--formula", "-1")
	if err != nil {
		return nil
	}
	installedSet := map[string]bool{}
	byBase := map[string][]string{}
	for _, n := range strings.Fields(out) {
		installedSet[n] = true
		if m := atVersion.FindStringSubmatch(n); m != nil {
			byBase[m[1]] = append(byBase[m[1]], n)
		}
	}
	running := map[string]bool{}
	if s, err := sh.Quiet("brew", "services", "list", "--json"); err == nil {
		var svcs []struct{ Name, Status string }
		if json.Unmarshal([]byte(s), &svcs) == nil {
			for _, sv := range svcs {
				if sv.Status == "started" {
					running[sv.Name] = true
				}
			}
		}
	}
	cellar := sh.Line("brew", "--cellar")

	var cands []string
	newest := map[string]string{}
	for base, names := range byBase {
		sort.Slice(names, func(i, j int) bool {
			return compareNums(versionNums(atVersion.FindStringSubmatch(names[i])[2]), versionNums(atVersion.FindStringSubmatch(names[j])[2])) > 0
		})
		// Keep the newest versioned one unless the plain formula (always newest) is installed.
		start := 1
		newest[base] = names[0]
		if installedSet[base] {
			start, newest[base] = 0, base
		}
		for _, n := range names[start:] {
			if running[n] || pinnedByProject(n, pins) {
				continue
			}
			cands = append(cands, n)
		}
	}

	items := make([]*Item, len(cands))
	Parallel(len(cands), func(i int) {
		n := cands[i]
		uses, err := sh.Quiet("brew", "uses", "--installed", n)
		if err != nil || strings.TrimSpace(uses) != "" {
			return // needed by something, or we can't tell: keep it
		}
		base := atVersion.FindStringSubmatch(n)[1]
		items[i] = &Item{Path: filepath.Join(cellar, n), Cmd: []string{"brew", "uninstall", n},
			Note: "Homebrew — nothing depends on it; keeping " + newest[base]}
	})
	var res []Item
	for _, it := range items {
		if it != nil {
			res = append(res, *it)
		}
	}
	return res
}

// brewTools maps versioned formula names to the tool their project pins use.
var brewTools = map[string]string{"php": "php", "python": "python", "node": "node", "openjdk": "java", "ruby": "ruby", "go": "go", "terraform": "terraform"}

func pinnedByProject(formula string, pins map[string][]string) bool {
	m := atVersion.FindStringSubmatch(formula)
	tool, ok := brewTools[m[1]]
	if !ok {
		return false
	}
	for _, p := range pins[tool] {
		if matchesPin(m[2], p) {
			return true
		}
	}
	return false
}

// ---------- Xcode, installers, models ----------

// ExtraXcodes returns Xcode copies other than the selected one.
func ExtraXcodes() []Item {
	apps, _ := filepath.Glob("/Applications/Xcode*.app")
	if len(apps) < 2 {
		return nil
	}
	selected := sh.Line("xcode-select", "-p") // …/Xcode.app/Contents/Developer
	var out []Item
	for _, a := range apps {
		if !strings.HasPrefix(selected, a+"/") {
			out = append(out, Item{Path: a, App: true, Note: "not the selected Xcode (" + filepath.Base(strings.TrimSuffix(selected, "/Contents/Developer")) + ")"})
		}
	}
	return out
}

// MacOSInstallers returns "Install macOS …" apps left in /Applications.
func MacOSInstallers() []Item {
	apps, _ := filepath.Glob("/Applications/Install macOS*.app")
	var out []Item
	for _, a := range apps {
		out = append(out, Item{Path: a, App: true, Note: "re-downloadable from the App Store"})
	}
	return out
}

// AIModels lists locally downloaded AI models.
func AIModels() []Item {
	var out []Item
	if fsx.IsDir(fsx.H(".ollama/models/blobs")) {
		out = append(out, Item{Path: fsx.H(".ollama/models"), Note: "all Ollama models (re-pull with `ollama pull`)"})
	}
	for _, pat := range []string{".lmstudio/models/*/*", ".cache/lm-studio/models/*/*", ".cache/huggingface/hub/models--*"} {
		ms, _ := filepath.Glob(fsx.H(pat))
		for _, m := range ms {
			if size := fsx.DiskUsage(m).Bytes; size > 0 {
				out = append(out, Item{Path: m, Size: size, Note: "last changed " + humanAgo(fsx.ModTime(m))})
			}
		}
	}
	return out
}

// OldVersions runs every version tool plus Homebrew and Xcode and returns what can go.
func OldVersions() []Item {
	pins := ProjectPins(ProjectRoots())
	tools := append(append([]versionTool(nil), versionTools...), pluginTools()...)
	results := make([][]Item, len(tools)+2)
	var wg sync.WaitGroup
	for i, t := range tools {
		wg.Add(1)
		go func(i int, t versionTool) { defer wg.Done(); results[i] = t.old(pins) }(i, t)
	}
	wg.Add(2)
	go func() { defer wg.Done(); results[len(tools)] = OldBrewVersions(pins) }()
	go func() { defer wg.Done(); results[len(tools)+1] = ExtraXcodes() }()
	wg.Wait()
	var out []Item
	for _, r := range results {
		out = append(out, r...)
	}
	return out
}
