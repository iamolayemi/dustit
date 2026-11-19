package finder

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iamolayemi/dustit/internal/docker"
	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/sh"
)

// Insight is a space user dustit reports but can't clean blindly.
type Insight struct {
	Text string `json:"text"`
	Hint string `json:"hint"`
}

// Insights reports the Docker disk image, Time Machine local snapshots and big app-data folders.
func Insights(human func(int64) string) []Insight {
	var out []Insight
	if used, _ := docker.RawSizes(); used > 1e9 {
		out = append(out, Insight{
			Text: fmt.Sprintf("Docker Desktop's disk image uses %s.", human(used)),
			Hint: "See what's inside with `dustit docker`; prune it with `dustit docker --clean`.",
		})
	}
	orb := fsx.H("Library/Group Containers/HUAQ24HBR6.dev.orbstack")
	if fsx.Exists(orb) {
		img := filepath.Join(orb, "data/data.img")
		if fsx.MayRead(img) {
			if u := fsx.DiskUsage(img).Bytes; u > 1e9 {
				out = append(out, Insight{
					Text: fmt.Sprintf("OrbStack's disk image uses %s.", human(u)),
					Hint: "OrbStack works with the docker CLI: `dustit docker --clean` prunes it too.",
				})
			}
		} else {
			out = append(out, Insight{
				Text: "OrbStack is installed; its disk image can't be measured without Full Disk Access.",
				Hint: "Prune it with `dustit docker --clean` (OrbStack works with the docker CLI).",
			})
		}
	}
	if s, err := sh.Quiet("tmutil", "listlocalsnapshots", "/"); err == nil {
		if n := strings.Count(s, "com.apple.TimeMachine"); n > 0 {
			out = append(out, Insight{
				Text: fmt.Sprintf("%d Time Machine local snapshot(s) may be holding space from deleted files.", n),
				Hint: "macOS removes them when space runs low; to remove now: tmutil thinlocalsnapshots / 999999999999 4",
			})
		}
	}
	if items, _, err := Children(fsx.H("Library/Application Support")); err == nil {
		var big []string
		for _, it := range items {
			if it.Size < 2e9 || len(big) == 5 {
				break
			}
			big = append(big, fmt.Sprintf("%s %s", filepath.Base(it.Path), human(it.Size)))
		}
		if len(big) > 0 {
			out = append(out, Insight{
				Text: "Largest app data folders: " + strings.Join(big, ", ") + ".",
				Hint: "This is app data, not cache: clean it from inside each app. Drill in with `dustit top \"~/Library/Application Support/<app>\"`.",
			})
		}
	}
	return out
}
