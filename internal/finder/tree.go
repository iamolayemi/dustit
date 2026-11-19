package finder

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/iamolayemi/macclean/internal/fsx"
)

// Node is a folder (or file) with its size and, for the first levels, its biggest children.
type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Size     int64   `json:"size"`
	Children []*Node `json:"children,omitempty"`
}

// Tree measures root once and keeps the `keep` biggest entries at depth 1 and 2
// (smaller ones are folded into an "other" node).
func Tree(root string, keep int) *Node {
	entries, _ := os.ReadDir(root)
	top := make([]*Node, len(entries))
	Parallel(len(entries), func(i int) {
		p := filepath.Join(root, entries[i].Name())
		n := &Node{Name: entries[i].Name(), Path: p}
		children := map[string]*Node{}
		_ = filepath.WalkDir(p, func(q string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() && q != p {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() && !fsx.MayRead(q) {
				return fs.SkipDir
			}
			var st syscall.Stat_t
			if syscall.Lstat(q, &st) != nil {
				return nil
			}
			size := st.Blocks * 512
			n.Size += size
			if q != p {
				rel := strings.TrimPrefix(q, p+"/")
				first := strings.SplitN(rel, "/", 2)[0]
				c := children[first]
				if c == nil {
					c = &Node{Name: first, Path: filepath.Join(p, first)}
					children[first] = c
				}
				c.Size += size
			}
			return nil
		})
		for _, c := range children {
			n.Children = append(n.Children, c)
		}
		n.Children = fold(n.Children, keep, p)
		top[i] = n
	})
	r := &Node{Name: fsx.Pretty(root), Path: root, Children: fold(top, keep, root)}
	for _, c := range r.Children {
		r.Size += c.Size
	}
	return r
}

// fold keeps the biggest `keep` nodes and merges the rest into "other".
func fold(nodes []*Node, keep int, parent string) []*Node {
	var nonEmpty []*Node
	for _, n := range nodes {
		if n != nil && n.Size > 0 {
			nonEmpty = append(nonEmpty, n)
		}
	}
	sort.Slice(nonEmpty, func(i, j int) bool { return nonEmpty[i].Size > nonEmpty[j].Size })
	if len(nonEmpty) <= keep {
		return nonEmpty
	}
	other := &Node{Name: "other", Path: parent}
	for _, n := range nonEmpty[keep:] {
		other.Size += n.Size
	}
	return append(nonEmpty[:keep], other)
}
