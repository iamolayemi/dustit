package targets

import (
	"fmt"
	"time"

	"github.com/iamolayemi/dustit/internal/history"
)

// Regrowth describes how fast a category fills back up after it was cleaned.
type Regrowth struct {
	CleanedAt time.Time `json:"cleaned_at"`
	Freed     int64     `json:"freed_bytes"`
	PerDay    int64     `json:"per_day_bytes"`
	Fast      bool      `json:"refills_fast"` // most of it came back within a few days
}

// Regrowths looks up, for each scanned category, its last clean-up in the history log and
// compares it with the size now.
func Regrowths(results []Result) map[string]Regrowth {
	recs, err := history.Load()
	if err != nil || len(recs) == 0 {
		return nil
	}
	type last struct {
		run   string
		at    time.Time
		freed int64
	}
	byCat := map[string]last{}
	for _, r := range recs {
		if r.Category == "" || (r.Action != history.Deleted && r.Action != history.Trashed) {
			continue
		}
		l := byCat[r.Category]
		if r.Run != l.run { // a newer run replaces the previous total
			if r.Time.Before(l.at) {
				continue
			}
			l = last{run: r.Run, at: r.Time}
		}
		l.freed += r.Size
		byCat[r.Category] = l
	}
	out := map[string]Regrowth{}
	now := time.Now()
	for _, res := range results {
		l, ok := byCat[res.T.ID]
		if !ok || l.freed < 50e6 || res.Bytes == 0 {
			continue
		}
		days := now.Sub(l.at).Hours() / 24
		if days < 0.25 {
			continue // cleaned just now
		}
		g := Regrowth{CleanedAt: l.at, Freed: l.freed, PerDay: int64(float64(res.Bytes) / days)}
		g.Fast = days <= 7 && res.Bytes >= l.freed/2
		out[res.T.ID] = g
	}
	return out
}

// Describe is a short note like "refills ~400 MB/day; cleaned 2 days ago".
func (g Regrowth) Describe(human func(int64) string) string {
	days := int(time.Since(g.CleanedAt).Hours() / 24)
	ago := "today"
	switch {
	case days == 1:
		ago = "yesterday"
	case days > 1:
		ago = fmt.Sprintf("%d days ago", days)
	}
	return fmt.Sprintf("refills ~%s/day; cleaned %s", human(g.PerDay), ago)
}
