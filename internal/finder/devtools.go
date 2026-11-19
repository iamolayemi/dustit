package finder

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/iamolayemi/macclean/internal/fsx"
)

// ElectronCaches finds Chromium/Electron cache folders inside ~/Library/Application Support.
func ElectronCaches() []Item {
	base := fsx.H("Library/Application Support")
	patterns := []string{
		"*/Cache", "*/Code Cache", "*/GPUCache", "*/DawnCache", "*/DawnGraphiteCache",
		"*/DawnWebGPUCache", "*/GrShaderCache", "*/ShaderCache",
		"*/Service Worker/CacheStorage", "*/Service Worker/ScriptCache",
		"*/Partitions/*/Cache", "*/Partitions/*/Code Cache", "*/Partitions/*/GPUCache",
		"*/Partitions/*/Service Worker/CacheStorage",
		"*/CachedData", "*/CachedExtensionVSIXs", // VS Code, Cursor, Windsurf…
		"Google/Chrome/*/Service Worker/CacheStorage", "Google/Chrome/*/GPUCache",
		"BraveSoftware/Brave-Browser/*/Service Worker/CacheStorage",
		"Microsoft Edge/*/Service Worker/CacheStorage",
		"Arc/User Data/*/Service Worker/CacheStorage",
	}
	seen := map[string]bool{}
	var out []Item
	for _, pat := range patterns {
		matches, _ := filepath.Glob(filepath.Join(base, pat))
		for _, m := range matches {
			if fi, err := os.Lstat(m); err == nil && fi.IsDir() && !seen[m] {
				seen[m] = true
				out = append(out, Item{Path: m})
			}
		}
	}
	return out
}

var jbRe = regexp.MustCompile(`^([A-Za-z]+?)(\d{4})\.(\d+)(.*)$`)

// JetBrainsOld returns per-version JetBrains folders older than the newest one of each IDE,
// plus "-backup" copies. IDEs with a single version are never touched.
func JetBrainsOld() []Item {
	base := fsx.H("Library/Application Support/JetBrains")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	type version struct {
		name, product, suffix string
		n                     int
	}
	var vs []version
	latest := map[string]int{}
	for _, e := range entries {
		m := jbRe.FindStringSubmatch(e.Name())
		if m == nil || !e.IsDir() {
			continue
		}
		y, _ := strconv.Atoi(m[2])
		mi, _ := strconv.Atoi(m[3])
		v := version{e.Name(), m[1], m[4], y*100 + mi}
		vs = append(vs, v)
		if v.suffix == "" && v.n > latest[v.product] {
			latest[v.product] = v.n
		}
	}
	var out []Item
	for _, v := range vs {
		l := latest[v.product]
		if l > 0 && (v.n < l || (v.suffix != "" && v.n <= l)) {
			out = append(out, Item{Path: filepath.Join(base, v.name)})
		}
	}
	return out
}
