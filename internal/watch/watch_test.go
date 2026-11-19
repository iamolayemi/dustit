package watch

import (
	"fmt"
	"testing"
	"time"

	"github.com/iamolayemi/dustit/internal/doctor"
	"github.com/iamolayemi/dustit/internal/timeline"
)

func human(n int64) string { return fmt.Sprintf("%d", n) }

func TestRunawayProcessNeedsThirtyMinutes(t *testing.T) {
	s := State{Hot: map[string]proc{}, Notified: map[string]time.Time{}}
	start := time.Now()
	hot := []doctor.Process{{PID: 42, CPU: 300, Path: "/opt/homebrew/bin/mysqld"}}
	for i := 0; i <= 2; i++ { // 0, 15, 30 minutes
		alerts := Evaluate(&s, Inputs{Now: start.Add(time.Duration(i) * 15 * time.Minute), Processes: hot, Free: 100e9}, human)
		if i < 2 && len(alerts) != 0 {
			t.Fatalf("no alert before 30 minutes (check %d): %+v", i, alerts)
		}
		if i == 2 && (len(alerts) != 1 || alerts[0].Key != "cpu:mysqld") {
			t.Fatalf("want a mysqld alert at 30 minutes, got %+v", alerts)
		}
	}
	// it cools down: state is cleared
	Evaluate(&s, Inputs{Now: start.Add(45 * time.Minute), Processes: nil, Free: 100e9}, human)
	if len(s.Hot) != 0 {
		t.Error("cooled-down process should be forgotten")
	}
}

func TestFreeSpaceDrop(t *testing.T) {
	s := State{Hot: map[string]proc{}, Notified: map[string]time.Time{}}
	now := time.Now()
	Evaluate(&s, Inputs{Now: now, Free: 100e9}, human)
	if a := Evaluate(&s, Inputs{Now: now.Add(30 * time.Minute), Free: 88e9}, human); len(a) != 1 || a[0].Key != "drop" {
		t.Fatalf("12 GB in 30 minutes should alert, got %+v", a)
	}
}

func TestGrowthAlerts(t *testing.T) {
	now := time.Now()
	prev := timeline.Snapshot{Time: now.Add(-24 * time.Hour), Folders: map[string]int64{"~/Projects/x": 1e9}}
	cur := timeline.Snapshot{Time: now, Folders: map[string]int64{"~/Projects/x": 8e9}}
	if a := GrowthAlerts(prev, cur, human); len(a) != 1 {
		t.Fatalf("7 GB in a day should alert, got %+v", a)
	}
}

func TestNotifyThrottle(t *testing.T) {
	Dir = t.TempDir()
	a := Alert{Key: "k"}
	now := time.Now()
	if !ShouldNotify(a, now) || ShouldNotify(a, now.Add(time.Hour)) || !ShouldNotify(a, now.Add(7*time.Hour)) {
		t.Error("one notification per key per 6 hours")
	}
}
