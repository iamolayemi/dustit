package targets

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iamolayemi/macclean/internal/finder"
	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/history"
	"github.com/iamolayemi/macclean/internal/sh"
)

// Result is what scanning one target found.
type Result struct {
	T           *Target
	Items       []finder.Item
	Bytes       int64
	Denied      int
	Unavailable string // why the target can't run here (e.g. tool not installed)
}

// SizeKnown is false for command targets whose effect can't be measured up front.
func (r Result) SizeKnown() bool { return len(r.T.CleanCmd) == 0 || len(r.T.PathCmd) > 0 }

// HasWork reports whether cleaning would do anything.
func (r Result) HasWork() bool {
	if r.Unavailable != "" {
		return false
	}
	if len(r.T.CleanCmd) > 0 {
		return !r.SizeKnown() || r.Bytes > 0
	}
	return len(r.Items) > 0
}

func excluded(name string, patterns []string) bool {
	if finder.SkipNames[name] {
		return true
	}
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, name); ok {
			return true
		}
	}
	return false
}

// Scan measures one target. It never removes anything.
func Scan(t *Target) Result {
	r := Result{T: t}
	if t.Needs != "" && !sh.Has(t.Needs) {
		r.Unavailable = t.Needs + " not installed"
		return r
	}
	if len(t.PathCmd) > 0 {
		p := sh.Line(t.PathCmd...)
		if !filepath.IsAbs(p) {
			// Report the failure instead of a misleading "0 B".
			r.Unavailable = "`" + strings.Join(t.PathCmd, " ") + "` failed"
			return r
		}
		u := fsx.DiskUsage(p)
		r.Bytes += u.Bytes
		r.Denied += u.Denied
	}

	var cutoff time.Time
	if t.MinAgeDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -t.MinAgeDays)
	}
	var items []finder.Item
	for _, dir := range t.Dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				r.Denied++
			}
			continue
		}
		for _, e := range entries {
			if !excluded(e.Name(), t.Exclude) {
				items = append(items, finder.Item{Path: filepath.Join(dir, e.Name())})
			}
		}
	}
	if t.Find != nil {
		items = append(items, t.Find()...)
	}
	kept := items[:0]
	for _, it := range items {
		if fsx.Excluded(it.Path) {
			continue
		}
		if it.Age.IsZero() {
			it.Age = fsx.ModTime(it.Path)
		}
		if cutoff.IsZero() || it.Age.Before(cutoff) {
			kept = append(kept, it)
		}
	}
	r.Denied += finder.Measure(kept)
	finder.SortBySize(kept)
	r.Items = kept
	r.Bytes += finder.Total(kept)
	return r
}

// ScanAll scans targets concurrently, keeping their order.
func ScanAll(ts []*Target) []Result {
	res := make([]Result, len(ts))
	finder.Parallel(len(ts), func(i int) { res[i] = Scan(ts[i]) })
	return res
}

// Totals adds up safe and review space, counting each path once even if two categories overlap.
func Totals(rs []Result) (safe, review int64, denied int) {
	counted := map[string]bool{}
	for _, r := range rs {
		denied += r.Denied
		add := r.Bytes
		if len(r.Items) > 0 {
			add = 0
			for _, it := range r.Items {
				if !counted[it.Path] {
					counted[it.Path] = true
					add += it.Size
				}
			}
		}
		if r.T.Risk == Safe {
			safe += add
		} else {
			review += add
		}
	}
	return
}

// CleanOptions controls Clean.
type CleanOptions struct {
	DryRun bool
	Trash  bool              // move items to Trash instead of deleting
	Log    func(line string) // progress lines (commands run, items that would be removed)
}

// Clean removes what a scan found and returns the space freed and the number of items skipped.
func Clean(r Result, o CleanOptions) (freed int64, failed int) {
	log := o.Log
	if log == nil {
		log = func(string) {}
	}
	t := r.T
	history.SetCategory(t.ID)
	defer history.SetCategory("")
	if len(t.CleanCmd) > 0 {
		log("$ " + joinArgs(t.CleanCmd))
		if o.DryRun {
			return r.Bytes, 0
		}
		if err := sh.Shown(t.CleanCmd...); err != nil {
			log("failed: " + err.Error())
			return 0, 1
		}
		if len(t.PathCmd) > 0 {
			return max(r.Bytes-Scan(t).Bytes, 0), 0
		}
		return 0, 0
	}
	for i, it := range r.Items {
		if !fsx.Exists(it.Path) && len(it.Cmd) == 0 {
			continue // already removed by an overlapping category
		}
		if o.DryRun {
			if i < 10 {
				line := "would remove " + fsx.Pretty(it.Path)
				if len(it.Cmd) > 0 {
					line = "would run " + displayCmd(it.Cmd)
				}
				if it.Note != "" {
					line += "  (" + it.Note + ")"
				}
				log(line)
			} else if i == 10 {
				log("… and " + itoa(len(r.Items)-10) + " more")
			}
			freed += it.Size
			continue
		}
		var err error
		switch {
		case len(it.Cmd) > 0:
			log("$ " + displayCmd(it.Cmd))
			if err = sh.Shown(it.Cmd...); err == nil {
				// Commands like git gc or brctl evict free only part of the size: measure.
				done := it.Size
				if fsx.Exists(it.Path) {
					done = max(it.Size-fsx.DiskUsage(it.Path).Bytes, 0)
				}
				history.Log(history.Deleted, it.Path, "", done)
				freed += done
				continue
			}
		case it.App:
			err = fsx.TrashApp(it.Path)
		default:
			err = fsx.RemoveSized(it.Path, o.Trash || t.ToTrash, it.Size)
		}
		if err != nil {
			failed++
			freed += max(it.Size-fsx.DiskUsage(it.Path).Bytes, 0) // partial removal still frees space
			continue
		}
		freed += it.Size
	}
	return freed, failed
}
