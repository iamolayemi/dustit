package finder

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sort"
	"sync"
	"syscall"

	"github.com/iamolayemi/macclean/internal/fsx"
)

// DupGroup is a set of files with identical content.
type DupGroup struct {
	Paths []string
	Size  int64
}

// Wasted is the space the extra copies take.
func (g DupGroup) Wasted() int64 { return g.Size * int64(len(g.Paths)-1) }

// Keeper is the index of the copy to keep: the oldest, then the shortest path.
func (g DupGroup) Keeper() int {
	best := 0
	bt := fsx.ModTime(g.Paths[0])
	for i, p := range g.Paths[1:] {
		t := fsx.ModTime(p)
		if t.Before(bt) || (t.Equal(bt) && len(p) < len(g.Paths[best])) {
			best, bt = i+1, t
		}
	}
	return best
}

func hashFile(p string, partial bool) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if partial { // first and last 16 KB: cheap and rules out most non-duplicates
		const chunk = 16 << 10
		buf := make([]byte, chunk)
		n, _ := io.ReadFull(f, buf)
		h.Write(buf[:n])
		if fi, err := f.Stat(); err == nil && fi.Size() > 2*chunk {
			if _, err := f.Seek(-chunk, io.SeekEnd); err == nil {
				n, _ = io.ReadFull(f, buf)
				h.Write(buf[:n])
			}
		}
	} else if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func groupBy(paths []string, key func(string) (string, error)) [][]string {
	m := map[string][]string{}
	var mu sync.Mutex
	Parallel(len(paths), func(i int) {
		k, err := key(paths[i])
		if err != nil {
			return
		}
		mu.Lock()
		m[k] = append(m[k], paths[i])
		mu.Unlock()
	})
	var out [][]string
	for _, g := range m {
		if len(g) > 1 {
			sort.Strings(g)
			out = append(out, g)
		}
	}
	return out
}

// Dupes finds identical files of at least minB: same size → same head/tail hash → same full hash.
func Dupes(roots []string, minB int64) []DupGroup {
	type inode struct{ dev, ino uint64 }
	seen := map[inode]bool{}
	bySize := map[int64][]string{}
	walkUserFiles(roots, func(p string, st *syscall.Stat_t) {
		if st.Size < minB {
			return
		}
		k := inode{uint64(st.Dev), st.Ino}
		if seen[k] { // hard link to a file we already have
			return
		}
		seen[k] = true
		bySize[st.Size] = append(bySize[st.Size], p)
	})
	var groups []DupGroup
	for size, ps := range bySize {
		if len(ps) < 2 {
			continue
		}
		for _, g1 := range groupBy(ps, func(p string) (string, error) { return hashFile(p, true) }) {
			for _, g2 := range groupBy(g1, func(p string) (string, error) { return hashFile(p, false) }) {
				groups = append(groups, DupGroup{g2, size})
			}
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Wasted() > groups[j].Wasted() })
	return groups
}

// DupeExtras lists every copy except the keeper of each group.
func DupeExtras(groups []DupGroup) []Item {
	var out []Item
	for _, g := range groups {
		k := g.Keeper()
		for i, p := range g.Paths {
			if i != k {
				out = append(out, Item{Path: p, Size: g.Size, Age: fsx.ModTime(p), Note: "copy of " + fsx.Pretty(g.Paths[k])})
			}
		}
	}
	SortBySize(out)
	return out
}
