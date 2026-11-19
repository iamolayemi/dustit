package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogAndUndo(t *testing.T) {
	Dir = t.TempDir()
	root := t.TempDir()
	orig := filepath.Join(root, "docs", "report.pdf")
	trash := filepath.Join(root, "Trash", "report.pdf")
	_ = os.MkdirAll(filepath.Dir(trash), 0o755)
	_ = os.WriteFile(trash, []byte("x"), 0o644)

	Begin("dustit clean")
	Log(Trashed, orig, trash, 100)
	Log(Deleted, filepath.Join(root, "cache"), "", 50)

	recs, err := Load()
	if err != nil || len(recs) != 2 {
		t.Fatalf("load: %v %d", err, len(recs))
	}
	runs := Runs(recs)
	if len(runs) != 1 || runs[0].Trashed != 1 || runs[0].Deleted != 1 || runs[0].Bytes != 150 {
		t.Fatalf("bad run %+v", runs)
	}
	r, err := Find(runs, "")
	if err != nil {
		t.Fatal(err)
	}
	res := Undo(r)
	if res.Restored != 1 || res.NotTrashed != 1 {
		t.Fatalf("undo %+v", res)
	}
	if _, err := os.Stat(orig); err != nil {
		t.Fatal("file not restored")
	}
	recs, _ = Load()
	if runs := Runs(recs); !findRun(runs, r.ID).Undone {
		t.Error("run should be marked undone")
	}
	if _, err := Find(Runs(recs), ""); err == nil {
		t.Error("nothing should be left to undo")
	}
}

func findRun(runs []Run, id string) Run {
	for _, r := range runs {
		if r.ID == id {
			return r
		}
	}
	return Run{}
}
