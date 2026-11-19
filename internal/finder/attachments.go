package finder

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/iamolayemi/dustit/internal/fsx"
)

// Attachments describes the attachments one app keeps.
type Attachments struct {
	App      string        `json:"app"`
	Paths    []string      `json:"paths"`
	Readable bool          `json:"readable"`
	Total    int64         `json:"total_bytes"`
	Files    int           `json:"files"`
	ByYear   map[int]int64 `json:"by_year"`
	Largest  []Item        `json:"largest"`
}

// AttachmentReport measures Messages and Mail attachments. These folders are protected
// by macOS (Full Disk Access) and are only ever read, never changed: deleting files inside
// them directly would break the apps' databases.
func AttachmentReport(largest int) []Attachments {
	mail, _ := filepath.Glob(fsx.H("Library/Mail/V*"))
	sources := []Attachments{
		{App: "Messages", Paths: []string{fsx.H("Library/Messages/Attachments")}},
		{App: "Mail", Paths: mail},
	}
	for i := range sources {
		a := &sources[i]
		a.ByYear = map[int]int64{}
		var files []Item
		for _, root := range a.Paths {
			if _, err := os.ReadDir(root); err != nil {
				if errors.Is(err, fs.ErrPermission) {
					a.Readable = false
				}
				continue
			}
			a.Readable = true
			_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				// Mail keeps messages and attachments side by side; count only attachments.
				if a.App == "Mail" && !containsDir(p, "Attachments") {
					return nil
				}
				var st syscall.Stat_t
				if syscall.Lstat(p, &st) != nil {
					return nil
				}
				size := st.Blocks * 512
				t := fsx.AddedOrEdited(&st)
				a.Total += size
				a.Files++
				a.ByYear[t.Year()] += size
				files = append(files, Item{Path: p, Size: size, Age: t})
				return nil
			})
		}
		SortBySize(files)
		if len(files) > largest {
			files = files[:largest]
		}
		a.Largest = files
	}
	return sources
}

func containsDir(p, name string) bool {
	for dir := filepath.Dir(p); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == name {
			return true
		}
	}
	return false
}
