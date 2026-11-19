package report

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"math"
	"time"

	"github.com/iamolayemi/macclean/internal/finder"
)

//go:embed report.html.tmpl
var files embed.FS

// Category is one row of the cleanup table.
type Category struct {
	ID, Name, Desc, Risk string
	Bytes                int64
	Items                int
	Known                bool
}

// Finding is one doctor result.
type Finding struct {
	Check, Severity, Summary, Short string
	Advice                          []string
}

// Data is everything the report shows.
type Data struct {
	Generated    time.Time
	Version      string
	Host         string
	Free, Total  int64
	Safe, Review int64
	Groups       []finder.Group
	Categories   []Category
	Findings     []Finding
	Tree         *finder.Node
	Human        func(int64) string
}

// Tile is a positioned treemap tile.
type Tile struct {
	X, Y, W, H float64
	Name, Size string
	Title      string
	Class      string // colour class, shared with the storage bar legend
	Parent     bool   // a top-level folder (drawn behind its children)
	Label, Two bool   // show the name; show the size on a second line
}

// Segment is one slice of the storage bar.
type Segment struct {
	Name, Size, Hint, Class string
	Pct                     float64
}

const (
	mapW, mapH = 1000.0, 520.0
	colours    = 10
)

func tiles(root *finder.Node, human func(int64) string) []Tile {
	if root == nil {
		return nil
	}
	var out []Tile
	sizes := make([]int64, len(root.Children))
	for i, c := range root.Children {
		sizes[i] = c.Size
	}
	for _, r := range Squarify(sizes, 0, 0, mapW, mapH) {
		top := root.Children[r.Index]
		class := fmt.Sprintf("c%d", r.Index%colours)
		name := top.Name
		if name == "other" {
			name, class = "Everything else", "cx"
		}
		if len(top.Children) == 0 || r.W < 70 || r.H < 50 {
			out = append(out, leaf(r, name, top, class, human))
			continue
		}
		out = append(out, Tile{X: r.X, Y: r.Y, W: r.W, H: r.H, Name: name, Size: human(top.Size), Class: class,
			Title: fmt.Sprintf("%s — %s", top.Path, human(top.Size)), Parent: true, Label: true})
		inner := make([]int64, len(top.Children))
		for i, c := range top.Children {
			inner[i] = c.Size
		}
		for _, c := range Squarify(inner, r.X+3, r.Y+24, r.W-6, r.H-27) {
			ch := top.Children[c.Index]
			out = append(out, leaf(c, ch.Name, ch, class, human))
		}
	}
	return out
}

func leaf(r Rect, name string, n *finder.Node, class string, human func(int64) string) Tile {
	return Tile{X: r.X, Y: r.Y, W: r.W, H: r.H, Name: name, Size: human(n.Size), Class: class,
		Title: fmt.Sprintf("%s — %s", n.Path, human(n.Size)),
		Label: r.W > 56 && r.H > 20, Two: r.W > 56 && r.H > 38}
}

func segments(d Data) []Segment {
	if d.Total <= 0 || len(d.Groups) == 0 {
		return nil
	}
	var out []Segment
	i := 0
	for _, g := range d.Groups {
		pct := float64(g.Size) / float64(d.Total) * 100
		if pct < 0.4 {
			continue
		}
		out = append(out, Segment{Name: g.Name, Size: d.Human(g.Size), Hint: g.Hint, Class: fmt.Sprintf("c%d", i%colours), Pct: pct})
		i++
	}
	return append(out, Segment{Name: "Free", Size: d.Human(d.Free), Class: "free", Pct: float64(d.Free) / float64(d.Total) * 100})
}

// Render returns the HTML page.
func Render(d Data) ([]byte, error) {
	funcs := template.FuncMap{
		"human": d.Human,
		"pct": func(a, b int64) float64 {
			if b == 0 {
				return 0
			}
			return math.Round(float64(a)/float64(b)*1000) / 10
		},
		"short": func(s string, w float64) string {
			r := []rune(s) // cut by character, never inside a multi-byte one
			n := int(w / 7.2)
			if len(r) <= n {
				return s
			}
			if n < 4 {
				return ""
			}
			return string(r[:n-1]) + "…"
		},
		"add": func(vals ...float64) float64 {
			var t float64
			for _, v := range vals {
				t += v
			}
			return t
		},
	}
	t, err := template.New("report.html.tmpl").Funcs(funcs).ParseFS(files, "report.html.tmpl")
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	err = t.Execute(&b, map[string]any{"D": d, "Tiles": tiles(d.Tree, d.Human), "Segments": segments(d), "W": mapW, "H": mapH})
	return b.Bytes(), err
}
