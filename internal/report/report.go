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
	Check, Severity, Summary string
	Advice                   []string
}

// Data is everything the report shows.
type Data struct {
	Generated    time.Time
	Version      string
	Free, Total  int64
	Safe, Review int64
	Categories   []Category
	Findings     []Finding
	Tree         *finder.Node
	Human        func(int64) string
}

// Tile is a positioned treemap tile for the template.
type Tile struct {
	X, Y, W, H   float64
	Name, Title  string
	Fill, Text   string
	Label, Small bool
}

// textOn picks black or white text for readable contrast on a #rrggbb fill.
func textOn(hex string) string {
	var r, g, b int
	_, _ = fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	if 0.299*float64(r)+0.587*float64(g)+0.114*float64(b) > 150 {
		return "#1d1d1f"
	}
	return "#ffffff"
}

var palette = []string{"#4e79a7", "#f28e2b", "#59a14f", "#e15759", "#76b7b2", "#edc948", "#b07aa1", "#9c755f", "#ff9da7", "#bab0ac", "#86bcb6", "#d37295", "#a0cbe8"}

const mapW, mapH = 1000.0, 560.0

// tiles lays out the two-level tree: each top folder gets a colour, its children are drawn inside it.
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
		color := palette[r.Index%len(palette)]
		if len(top.Children) == 0 || r.W < 40 || r.H < 30 {
			out = append(out, tile(r, top.Name, fmt.Sprintf("%s — %s", top.Path, human(top.Size)), color, human))
			continue
		}
		// leave room for the folder's own label
		inner := make([]int64, len(top.Children))
		for i, c := range top.Children {
			inner[i] = c.Size
		}
		out = append(out, Tile{X: r.X, Y: r.Y, W: r.W, H: r.H, Name: top.Name + " · " + human(top.Size),
			Title: fmt.Sprintf("%s — %s", top.Path, human(top.Size)), Fill: color, Text: textOn(color), Label: true})
		for _, c := range Squarify(inner, r.X+2, r.Y+18, r.W-4, r.H-20) {
			ch := top.Children[c.Index]
			t := tile(c, ch.Name, fmt.Sprintf("%s — %s", ch.Path, human(ch.Size)), color, human)
			t.Small = true
			out = append(out, t)
		}
	}
	return out
}

func tile(r Rect, name, title, color string, human func(int64) string) Tile {
	return Tile{X: r.X, Y: r.Y, W: r.W, H: r.H, Name: name, Title: title, Fill: color, Text: textOn(color),
		Label: r.W > 60 && r.H > 18}
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
		"bar": func(a int64) float64 {
			var mx int64
			for _, c := range d.Categories {
				mx = max(mx, c.Bytes)
			}
			if mx == 0 {
				return 0
			}
			return float64(a) / float64(mx) * 100
		},
		"short": func(s string, w float64) string {
			r := []rune(s) // cut by character, never inside a multi-byte one
			n := int(w / 7)
			if len(r) <= n {
				return s
			}
			if n < 4 {
				return ""
			}
			return string(r[:n-1]) + "…"
		},
		"add": func(a, b float64) float64 { return a + b },
	}
	t, err := template.New("report.html.tmpl").Funcs(funcs).ParseFS(files, "report.html.tmpl")
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	err = t.Execute(&b, map[string]any{"D": d, "Tiles": tiles(d.Tree, d.Human), "W": mapW, "H": mapH})
	return b.Bytes(), err
}
