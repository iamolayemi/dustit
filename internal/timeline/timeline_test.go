package timeline

import (
	"testing"
	"time"
)

func TestSaveLoadAndBefore(t *testing.T) {
	Dir = t.TempDir()
	now := time.Now()
	for i := 3; i >= 0; i-- {
		if err := Save(Snapshot{Time: now.AddDate(0, 0, -i), Free: int64(100 - i)}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := Load()
	if err != nil || len(all) != 4 {
		t.Fatalf("load: %v %d", err, len(all))
	}
	s, ok := Before(all, now.AddDate(0, 0, -2).Add(time.Hour))
	if !ok || s.Free != 98 {
		t.Errorf("Before picked %+v", s)
	}
	if SaveIfStale(Snapshot{Time: now.Add(time.Minute)}, time.Hour) {
		t.Error("a snapshot a minute after the last one should be skipped")
	}
}

func TestDiffReportsTheMostSpecificFolder(t *testing.T) {
	a := Snapshot{Groups: map[string]int64{"Docker": 10e9, "Downloads": 5e9},
		Folders: map[string]int64{"~/Projects": 20e9, "~/Projects/app": 5e9, "~/Downloads": 5e9}}
	b := Snapshot{Groups: map[string]int64{"Docker": 22e9, "Downloads": 5e9},
		Folders: map[string]int64{"~/Projects": 24e9, "~/Projects/app": 9e9, "~/Downloads": 5e9}}
	groups, folders := Diff(a, b, 1e9)
	if len(groups) != 1 || groups[0].Name != "Docker" || groups[0].Delta != 12e9 {
		t.Errorf("groups: %+v", groups)
	}
	if len(folders) != 1 || folders[0].Name != "~/Projects/app" {
		t.Errorf("the child explains the parent's growth, so only it should be reported: %+v", folders)
	}
}
