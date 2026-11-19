// Package finder locates things worth cleaning. It never deletes anything.
package finder

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/iamolayemi/macclean/internal/fsx"
)

// Item is something that could be removed.
type Item struct {
	Path string
	Size int64
	Age  time.Time
	Note string
	App  bool     // an .app bundle in /Applications: removed with fsx.TrashApp
	Cmd  []string // removed by running this command instead of deleting Path
}

// Total sums item sizes.
func Total(items []Item) int64 {
	var t int64
	for _, it := range items {
		t += it.Size
	}
	return t
}

// SortBySize sorts largest first.
func SortBySize(items []Item) {
	sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
}

// Parallel runs fn(i) for i in [0,n) on a small worker pool.
func Parallel(n int, fn func(i int)) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(i)
		}(i)
	}
	wg.Wait()
}

// Measure fills in the size of every item whose size is unknown and returns denied count.
func Measure(items []Item) (denied int) {
	var mu sync.Mutex
	Parallel(len(items), func(i int) {
		if items[i].Size != 0 {
			return
		}
		u := fsx.DiskUsage(items[i].Path)
		items[i].Size = u.Bytes
		mu.Lock()
		denied += u.Denied
		mu.Unlock()
	})
	return denied
}

// SkipNames are Finder metadata files nobody wants listed.
var SkipNames = map[string]bool{".DS_Store": true, ".localized": true}

// Children measures each entry directly inside dir.
func Children(dir string) ([]Item, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}
	items := make([]Item, 0, len(entries))
	for _, e := range entries {
		items = append(items, Item{Path: filepath.Join(dir, e.Name())})
	}
	denied := Measure(items)
	SortBySize(items)
	return items, denied, nil
}

// OldEntries lists entries directly inside dir untouched for `days` days and at least minB in size.
func OldEntries(dir string, days int, minB int64) ([]Item, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	var items []Item
	for _, e := range entries {
		if SkipNames[e.Name()] {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if age := fsx.ModTime(p); age.Before(cutoff) {
			items = append(items, Item{Path: p, Age: age})
		}
	}
	Measure(items)
	out := items[:0]
	for _, it := range items {
		if it.Size >= minB {
			out = append(out, it)
		}
	}
	SortBySize(out)
	return out, nil
}

// LargeFiles finds files of at least minB anywhere under root.
func LargeFiles(root string, minB int64) []Item {
	var found []Item
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && p != root {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p == fsx.H("Library/Mobile Documents") { // iCloud placeholders
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		var st syscall.Stat_t
		if syscall.Lstat(p, &st) == nil && st.Blocks*512 >= minB {
			found = append(found, Item{Path: p, Size: st.Blocks * 512, Age: fsx.LastTouched(&st)})
		}
		return nil
	})
	SortBySize(found)
	return found
}

// ---------- where to look ----------

// DedupeRoots drops missing folders, case-insensitive duplicates and folders nested in another root.
func DedupeRoots(paths []string) []string {
	var ok []string
	seen := map[string]bool{}
	for _, p := range paths {
		if !fsx.IsDir(p) {
			continue
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		if k := strings.ToLower(p); !seen[k] {
			seen[k] = true
			ok = append(ok, p)
		}
	}
	var out []string
	for _, p := range ok {
		nested := false
		for _, q := range ok {
			if p != q && strings.HasPrefix(strings.ToLower(p), strings.ToLower(q)+"/") {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, p)
		}
	}
	return out
}

// ProjectRoots are the usual places code lives.
func ProjectRoots() []string {
	var c []string
	for _, n := range []string{"Projects", "Developer", "code", "src", "dev", "workspace", "Sites", "Documents", "Desktop"} {
		c = append(c, fsx.H(n))
	}
	return DedupeRoots(c)
}

// UserFileRoots are personal folders plus project folders.
func UserFileRoots(withDownloads bool) []string {
	c := []string{fsx.H("Desktop"), fsx.H("Documents"), fsx.H("Movies"), fsx.H("Music"), fsx.H("Pictures")}
	if withDownloads {
		c = append(c, fsx.H("Downloads"))
	}
	return DedupeRoots(append(c, ProjectRoots()...))
}

// appManaged are folders whose files belong to an app's library; removing them breaks the app.
func appManaged(p string) bool {
	switch p {
	case fsx.H("Library"), fsx.H("Music/Music"), fsx.H("Music/iTunes"), fsx.H("Pictures/Photo Booth Library"):
		return true
	}
	return false
}

var bundleSuffixes = []string{".app", ".photoslibrary", ".photolibrary", ".musiclibrary", ".tvlibrary", ".fcpbundle"}

func skipUserDir(p, name string) bool {
	if appManaged(p) || strings.HasPrefix(name, ".") || isArtifact(name) ||
		strings.EqualFold(name, "cache") || strings.EqualFold(name, "caches") {
		return true
	}
	for _, s := range bundleSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

// walkUserFiles calls fn for every regular file under roots, skipping app libraries, caches,
// hidden folders and dependency folders. It never follows symlinks.
func walkUserFiles(roots []string, fn func(p string, st *syscall.Stat_t)) {
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() && p != root {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if p != root && skipUserDir(p, d.Name()) {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() || SkipNames[d.Name()] {
				return nil
			}
			var st syscall.Stat_t
			if syscall.Lstat(p, &st) == nil {
				fn(p, &st)
			}
			return nil
		})
	}
}

// OldFiles finds files of at least minB not changed in `days` days.
func OldFiles(roots []string, days int, minB int64) []Item {
	cutoff := time.Now().AddDate(0, 0, -days)
	var out []Item
	walkUserFiles(roots, func(p string, st *syscall.Stat_t) {
		size := st.Blocks * 512
		if t := fsx.LastTouched(st); size >= minB && t.Before(cutoff) {
			out = append(out, Item{Path: p, Size: size, Age: t, Note: "last changed " + humanAgo(t)})
		}
	})
	SortBySize(out)
	return out
}
