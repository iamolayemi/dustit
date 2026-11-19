package finder

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iamolayemi/macclean/internal/docker"
	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/sh"
)

// Insight is a space user macclean reports but can't clean blindly.
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
			Hint: "See what's inside with `macclean docker`; prune it with `macclean docker --clean`.",
		})
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
				Hint: "This is app data, not cache: clean it from inside each app. Drill in with `macclean top \"~/Library/Application Support/<app>\"`.",
			})
		}
	}
	return out
}
