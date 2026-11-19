// Package timeline records how disk usage changes over time, so DustIt can say what grew.
package timeline

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/iamolayemi/dustit/internal/finder"
	"github.com/iamolayemi/dustit/internal/fsx"
)

// Snapshot is one measurement of the disk.
type Snapshot struct {
	Time    time.Time        `json:"time"`
	Free    int64            `json:"free"`
	Total   int64            `json:"total"`
	Groups  map[string]int64 `json:"groups"`
	Folders map[string]int64 `json:"folders"` // ~-relative paths, one and two levels deep
}

// minFolder is the smallest folder worth tracking.
const minFolder = 50e6

// keep is how many snapshots are kept (about a year of daily ones).
const keep = 400

// Dir can be overridden in tests.
var Dir = func() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Library", "Application Support", "dustit")
}()

// File is where snapshots are stored.
func File() string { return filepath.Join(Dir, "timeline.jsonl") }

// FromTree turns an already-measured home tree (built with a large keep) into a snapshot.
func FromTree(tree *finder.Node) Snapshot {
	free, total := fsx.DiskTotals()
	s := Snapshot{Time: time.Now(), Free: free, Total: total, Groups: map[string]int64{}, Folders: map[string]int64{}}
	for _, g := range finder.StorageFrom(tree) {
		s.Groups[g.Name] = g.Size
	}
	for _, top := range tree.Children {
		if top.Size >= minFolder {
			s.Folders[fsx.Pretty(top.Path)] = top.Size
		}
		for _, c := range top.Children {
			if c.Size >= minFolder {
				s.Folders[fsx.Pretty(c.Path)] = c.Size
			}
		}
	}
	return s
}

// Take measures the disk now.
func Take() Snapshot { return FromTree(finder.Tree(fsx.Home, 100000)) }

// Load reads every snapshot, oldest first.
func Load() ([]Snapshot, error) {
	f, err := os.Open(File())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []Snapshot
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		var s Snapshot
		if json.Unmarshal(sc.Bytes(), &s) == nil {
			out = append(out, s)
		}
	}
	return out, sc.Err()
}

// Save appends a snapshot, trimming the file to the most recent `keep`.
func Save(s Snapshot) error {
	all, _ := Load()
	all = append(all, s)
	if len(all) > keep {
		all = all[len(all)-keep:]
	}
	if err := os.MkdirAll(Dir, 0o755); err != nil {
		return err
	}
	tmp := File() + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, x := range all {
		b, _ := json.Marshal(x)
		_, _ = w.Write(append(b, '\n'))
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, File())
}

// SaveIfStale records s unless a snapshot was saved within `gap`. It reports whether it saved.
func SaveIfStale(s Snapshot, gap time.Duration) bool {
	all, _ := Load()
	if len(all) > 0 && s.Time.Sub(all[len(all)-1].Time) < gap {
		return false
	}
	return Save(s) == nil
}

// Before returns the newest snapshot taken at or before t (the oldest one if none is).
func Before(all []Snapshot, t time.Time) (Snapshot, bool) {
	if len(all) == 0 {
		return Snapshot{}, false
	}
	best := all[0]
	for _, s := range all {
		if s.Time.After(t) {
			break
		}
		best = s
	}
	return best, true
}

// Change is how much something grew (positive) or shrank (negative).
type Change struct {
	Name   string `json:"name"`
	Before int64  `json:"before"`
	After  int64  `json:"after"`
	Delta  int64  `json:"delta"`
}

// Diff compares two snapshots: group changes, and folder changes where a folder's own
// growth isn't just its biggest child's (so the most specific folder is reported).
func Diff(a, b Snapshot, minDelta int64) (groups, folders []Change) {
	groups = diffMaps(a.Groups, b.Groups, minDelta)
	all := diffMaps(a.Folders, b.Folders, minDelta)
	for _, c := range all {
		explained := false
		for _, d := range all {
			if d.Name != c.Name && filepath.Dir(d.Name) == c.Name && abs(d.Delta) >= abs(c.Delta)*8/10 {
				explained = true // a child accounts for most of it; report the child instead
				break
			}
		}
		if !explained {
			folders = append(folders, c)
		}
	}
	return groups, folders
}

func diffMaps(a, b map[string]int64, minDelta int64) []Change {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	var out []Change
	for k := range keys {
		d := b[k] - a[k]
		if abs(d) >= minDelta {
			out = append(out, Change{Name: k, Before: a[k], After: b[k], Delta: d})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Delta > out[j].Delta })
	return out
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
