// Package history keeps a log of everything DustIt removes, so runs can be
// reviewed and items that went to the Trash can be put back.
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Actions recorded in the log.
const (
	Deleted  = "deleted"
	Trashed  = "trashed"
	Restored = "restored"
)

// Record is one line of the log.
type Record struct {
	Run       string    `json:"run"`
	Time      time.Time `json:"time"`
	Command   string    `json:"command"`
	Category  string    `json:"category,omitempty"`
	Action    string    `json:"action"`
	Path      string    `json:"path"`
	TrashPath string    `json:"trash_path,omitempty"`
	Size      int64     `json:"size"`
	Of        string    `json:"of,omitempty"` // for Restored: the run that was undone
}

var (
	mu       sync.Mutex
	run      = newRunID()
	command  = "dustit"
	category string
	// Dir can be overridden in tests.
	Dir = defaultDir()
)

func defaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "dustit")
}

func newRunID() string { return time.Now().Format("20060102-150405.000") }

// File is the log location.
func File() string { return filepath.Join(Dir, "history.jsonl") }

// Begin starts a new run for a command (called once per invocation).
func Begin(cmd string) {
	mu.Lock()
	defer mu.Unlock()
	run, command, category = newRunID(), cmd, ""
}

// SetCategory labels the records that follow.
func SetCategory(c string) {
	mu.Lock()
	category = c
	mu.Unlock()
}

// Log appends a record for the current run. Logging failures never block a cleanup.
func Log(action, path, trashPath string, size int64) {
	mu.Lock()
	r := Record{Run: run, Time: time.Now(), Command: command, Category: category,
		Action: action, Path: path, TrashPath: trashPath, Size: size}
	mu.Unlock()
	_ = appendRecord(r)
}

func appendRecord(r Record) error {
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(Dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(File(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	b, _ := json.Marshal(r)
	_, err = f.Write(append(b, '\n'))
	return err
}

// Load reads every record (oldest first).
func Load() ([]Record, error) {
	f, err := os.Open(File())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var r Record
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out, sc.Err()
}

// Run groups the records of one invocation.
type Run struct {
	ID       string    `json:"id"`
	Command  string    `json:"command"`
	Time     time.Time `json:"time"`
	Records  []Record  `json:"-"`
	Undone   bool      `json:"undone"`
	Bytes    int64     `json:"size_bytes"`
	Trashed  int       `json:"trashed"`
	Deleted  int       `json:"deleted"`
	Restored int       `json:"restored,omitempty"` // for undo runs
}

// Runs groups records into runs, newest first.
func Runs(records []Record) []Run {
	byID := map[string]*Run{}
	undone := map[string]bool{}
	for _, r := range records {
		run, ok := byID[r.Run]
		if !ok {
			run = &Run{ID: r.Run, Command: r.Command, Time: r.Time}
			byID[r.Run] = run
		}
		run.Records = append(run.Records, r)
		switch r.Action {
		case Trashed:
			run.Trashed++
			run.Bytes += r.Size
		case Deleted:
			run.Deleted++
			run.Bytes += r.Size
		case Restored:
			run.Restored++
			undone[r.Of] = true
		}
	}
	out := make([]Run, 0, len(byID))
	for id, r := range byID {
		r.Undone = undone[id]
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// Find returns the run with the given id, or the newest undoable run when id is empty.
func Find(runs []Run, id string) (Run, error) {
	for _, r := range runs {
		if id == "" && r.Trashed > 0 && !r.Undone {
			return r, nil
		}
		if id != "" && r.ID == id {
			return r, nil
		}
	}
	if id == "" {
		return Run{}, errors.New("nothing to undo: no run has items in the Trash")
	}
	return Run{}, fmt.Errorf("no run with id %q (see: dustit history)", id)
}

// UndoResult reports what Undo did.
type UndoResult struct {
	Restored   int
	Bytes      int64
	Missing    []string // no longer in the Trash
	Occupied   []string // something new exists at the original path
	NotTrashed int      // permanently deleted items can't come back
	Errors     []error
}

// Undo moves a run's trashed items back to where they were.
func Undo(r Run) UndoResult {
	var res UndoResult
	Begin("dustit undo " + r.ID)
	for _, rec := range r.Records {
		switch {
		case rec.Action == Deleted:
			res.NotTrashed++
			continue
		case rec.Action != Trashed:
			continue
		}
		if _, err := os.Lstat(rec.TrashPath); err != nil {
			res.Missing = append(res.Missing, rec.Path)
			continue
		}
		if _, err := os.Lstat(rec.Path); err == nil {
			res.Occupied = append(res.Occupied, rec.Path)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(rec.Path), 0o755); err != nil {
			res.Errors = append(res.Errors, err)
			continue
		}
		if err := os.Rename(rec.TrashPath, rec.Path); err != nil {
			res.Errors = append(res.Errors, err)
			continue
		}
		res.Restored++
		res.Bytes += rec.Size
		mu.Lock()
		rr := Record{Run: run, Time: time.Now(), Command: command, Action: Restored,
			Path: rec.Path, TrashPath: rec.TrashPath, Size: rec.Size, Of: r.ID}
		mu.Unlock()
		_ = appendRecord(rr)
	}
	return res
}
