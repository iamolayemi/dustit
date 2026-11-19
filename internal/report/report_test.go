package report

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/iamolayemi/macclean/internal/finder"
)

func TestSquarifyFillsAreaProportionally(t *testing.T) {
	sizes := []int64{600, 300, 100, 0, 50}
	rects := Squarify(sizes, 0, 0, 100, 50)
	if len(rects) != 4 {
		t.Fatalf("zero sizes are skipped: got %d rects", len(rects))
	}
	var area float64
	for _, r := range rects {
		area += r.W * r.H
		want := float64(sizes[r.Index]) / 1050 * 5000
		if math.Abs(r.W*r.H-want) > 0.01 {
			t.Errorf("tile %d area %.2f, want %.2f", r.Index, r.W*r.H, want)
		}
		if r.X < -1e-9 || r.Y < -1e-9 || r.X+r.W > 100+1e-9 || r.Y+r.H > 50+1e-9 {
			t.Errorf("tile %d outside the box: %+v", r.Index, r)
		}
	}
	if math.Abs(area-5000) > 0.01 {
		t.Errorf("tiles cover %.2f, want 5000", area)
	}
}

func TestRenderEscapesAndIncludesSections(t *testing.T) {
	human := func(n int64) string { return fmt.Sprintf("%d B", n) }
	tree := &finder.Node{Name: "~", Path: "/h", Size: 300, Children: []*finder.Node{
		{Name: "<script>alert(1)</script>", Path: "/h/x", Size: 200},
		{Name: "Projects · with a very long name ünïcödé that gets shortened", Path: "/h/p", Size: 100},
	}}
	html, err := Render(Data{Generated: time.Now(), Version: "test", Free: 10, Total: 100, Safe: 5, Review: 1,
		Categories: []Category{{ID: "npm-cache", Risk: "safe", Bytes: 5, Known: true}},
		Findings:   []Finding{{Check: "Memory", Severity: "warn", Summary: "high"}}, Tree: tree, Human: human})
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	if !utf8.Valid(html) {
		t.Error("report must be valid UTF-8")
	}
	if strings.Contains(s, "<script>alert(1)</script>") {
		t.Error("folder names must be escaped")
	}
	for _, want := range []string{"npm-cache", "Memory:", "<svg", "Projects"} {
		if !strings.Contains(s, want) {
			t.Errorf("report is missing %q", want)
		}
	}
}
