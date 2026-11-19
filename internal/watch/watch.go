// Package watch spots runaway processes and sudden disk growth between periodic checks.
package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/iamolayemi/dustit/internal/doctor"
	"github.com/iamolayemi/dustit/internal/timeline"
)

// Thresholds.
const (
	HotCPU      = 80.0             // percent of one core
	HotFor      = 30 * time.Minute // sustained this long
	gapAllowed  = 45 * time.Minute // checks further apart than this restart the clock
	DropBytes   = 10e9             // free space lost…
	DropWindow  = 90 * time.Minute // …within this window
	GrowBytes   = 5e9              // a folder growing this much in a day
	SnapshotGap = 20 * time.Hour
	notifyEvery = 6 * time.Hour
)

type proc struct {
	Name      string    `json:"name"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	CPU       float64   `json:"cpu"`
}

// State persists between checks.
type State struct {
	Hot      map[string]proc      `json:"hot"` // by pid
	Free     int64                `json:"free"`
	FreeAt   time.Time            `json:"free_at"`
	Notified map[string]time.Time `json:"notified"`
}

// Alert is something worth telling the user about.
type Alert struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Dir can be overridden in tests.
var Dir = timeline.Dir

func stateFile() string { return filepath.Join(Dir, "watch-state.json") }

func load() State {
	s := State{Hot: map[string]proc{}, Notified: map[string]time.Time{}}
	if b, err := os.ReadFile(stateFile()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.Hot == nil {
		s.Hot = map[string]proc{}
	}
	if s.Notified == nil {
		s.Notified = map[string]time.Time{}
	}
	return s
}

func (s State) save() {
	_ = os.MkdirAll(Dir, 0o755)
	b, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(stateFile(), b, 0o644)
}

// Inputs is what one check looks at (separated out so it can be tested).
type Inputs struct {
	Now       time.Time
	Processes []doctor.Process
	Free      int64
}

// Evaluate updates the state with one observation and returns new alerts.
func Evaluate(s *State, in Inputs, human func(int64) string) []Alert {
	var alerts []Alert
	seen := map[string]bool{}
	for _, p := range in.Processes {
		if p.CPU < HotCPU {
			continue
		}
		key := strconv.Itoa(p.PID)
		seen[key] = true
		h, ok := s.Hot[key]
		if !ok || in.Now.Sub(h.LastSeen) > gapAllowed || h.Name != p.Name() {
			h = proc{Name: p.Name(), FirstSeen: in.Now}
		}
		h.LastSeen, h.CPU = in.Now, p.CPU
		s.Hot[key] = h
		if in.Now.Sub(h.FirstSeen) >= HotFor {
			alerts = append(alerts, Alert{Key: "cpu:" + h.Name,
				Title: h.Name + " is hogging the CPU",
				Body:  fmt.Sprintf("%s has used %.0f%% CPU for %d minutes. Run dustit doctor for advice.", h.Name, p.CPU, int(in.Now.Sub(h.FirstSeen).Minutes()))})
		}
	}
	for key := range s.Hot {
		if !seen[key] {
			delete(s.Hot, key)
		}
	}
	if s.Free > 0 && in.Now.Sub(s.FreeAt) <= DropWindow && s.Free-in.Free >= DropBytes {
		alerts = append(alerts, Alert{Key: "drop",
			Title: "Free space dropped fast",
			Body:  fmt.Sprintf("%s used in the last %d minutes. Run dustit growth to see where.", human(s.Free-in.Free), int(in.Now.Sub(s.FreeAt).Minutes()))})
	}
	if s.Free == 0 || in.Now.Sub(s.FreeAt) > DropWindow || in.Free > s.Free {
		s.Free, s.FreeAt = in.Free, in.Now // start a new window
	}
	return alerts
}

// GrowthAlerts compares a new snapshot with the previous one and flags fast-growing folders.
func GrowthAlerts(prev, cur timeline.Snapshot, human func(int64) string) []Alert {
	if prev.Time.IsZero() || cur.Time.Sub(prev.Time) > 50*time.Hour {
		return nil
	}
	_, folders := timeline.Diff(prev, cur, GrowBytes)
	var out []Alert
	for _, f := range folders {
		if f.Delta > 0 {
			out = append(out, Alert{Key: "grow:" + f.Name, Title: "A folder grew fast",
				Body: fmt.Sprintf("%s grew %s since %s.", f.Name, human(f.Delta), prev.Time.Format("Mon 15:04"))})
		}
	}
	return out
}

// Check runs one watch cycle: processes, free space and (once a day) a snapshot.
func Check(now time.Time, free int64, human func(int64) string) []Alert {
	s := load()
	alerts := Evaluate(&s, Inputs{Now: now, Processes: doctor.Processes(), Free: free}, human)
	if all, _ := timeline.Load(); len(all) == 0 || now.Sub(all[len(all)-1].Time) >= SnapshotGap {
		cur := timeline.Take()
		if len(all) > 0 {
			alerts = append(alerts, GrowthAlerts(all[len(all)-1], cur, human)...)
		}
		_ = timeline.Save(cur)
	}
	s.save()
	return alerts
}

// ShouldNotify throttles each alert key to one notification per 6 hours.
func ShouldNotify(a Alert, now time.Time) bool {
	s := load()
	if t, ok := s.Notified[a.Key]; ok && now.Sub(t) < notifyEvery {
		return false
	}
	s.Notified[a.Key] = now
	s.save()
	return true
}
