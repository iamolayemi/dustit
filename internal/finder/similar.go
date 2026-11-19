package finder

import (
	"image"
	_ "image/gif" // register decoders
	_ "image/jpeg"
	_ "image/png"
	"math/bits"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
)

var nativeImage = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true}
var sipsImage = map[string]bool{".heic": true, ".heif": true, ".tif": true, ".tiff": true, ".webp": true}

// SimilarGroup is a set of images that look alike.
type SimilarGroup struct {
	Items []Item // largest first; the first is the suggested keeper
}

// Bytes the extra copies take.
func (g SimilarGroup) Extra() int64 {
	var t int64
	for _, it := range g.Items[1:] {
		t += it.Size
	}
	return t
}

// decodeSmall returns the image, converting formats Go can't read with macOS's sips.
func decodeSmall(p string) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(p))
	src := p
	if sipsImage[ext] {
		tmp, err := os.CreateTemp("", "dustit-thumb-*.png")
		if err != nil {
			return nil, err
		}
		_ = tmp.Close()
		defer func() { _ = os.Remove(tmp.Name()) }()
		if err := exec.Command("sips", "-s", "format", "png", "-Z", "128", p, "--out", tmp.Name()).Run(); err != nil {
			return nil, err
		}
		src = tmp.Name()
	}
	f, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	img, _, err := image.Decode(f)
	return img, err
}

// DHash is a 64-bit difference hash: shrink to 9×8 greyscale and compare neighbours.
// Similar-looking images get hashes that differ in few bits.
func DHash(img image.Image) uint64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 0
	}
	var grey [8][9]float64
	for y := 0; y < 8; y++ {
		for x := 0; x < 9; x++ {
			// average a block of source pixels (box filter)
			x0, x1 := b.Min.X+x*w/9, b.Min.X+(x+1)*w/9
			y0, y1 := b.Min.Y+y*h/8, b.Min.Y+(y+1)*h/8
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			step := max(1, (x1-x0)*(y1-y0)/64) // sample at most ~64 pixels per block
			var sum float64
			n := 0
			for i := 0; i < (x1-x0)*(y1-y0); i += step {
				px, py := x0+i%(x1-x0), y0+i/(x1-x0)
				r, g, bl, _ := img.At(px, py).RGBA()
				sum += 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)
				n++
			}
			grey[y][x] = sum / float64(n)
		}
	}
	var hash uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			hash <<= 1
			if grey[y][x] > grey[y][x+1] {
				hash |= 1
			}
		}
	}
	return hash
}

// SimilarImages groups images under roots whose hashes differ by at most `threshold` bits.
func SimilarImages(roots []string, threshold int, minB int64) []SimilarGroup {
	var paths []Item
	walkUserFiles(roots, func(p string, st *syscall.Stat_t) {
		ext := strings.ToLower(filepath.Ext(p))
		if (nativeImage[ext] || sipsImage[ext]) && st.Blocks*512 >= minB {
			paths = append(paths, Item{Path: p, Size: st.Blocks * 512, Age: fsxTime(st)})
		}
	})
	hashes := make([]uint64, len(paths))
	ok := make([]bool, len(paths))
	Parallel(len(paths), func(i int) {
		if img, err := decodeSmall(paths[i].Path); err == nil {
			hashes[i], ok[i] = DHash(img), true
		}
	})

	// union-find over every close pair
	parent := make([]int, len(paths))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	var mu sync.Mutex
	for i := range paths {
		if !ok[i] {
			continue
		}
		for j := i + 1; j < len(paths); j++ {
			if ok[j] && bits.OnesCount64(hashes[i]^hashes[j]) <= threshold {
				mu.Lock()
				parent[find(i)] = find(j)
				mu.Unlock()
			}
		}
	}
	groups := map[int][]Item{}
	for i := range paths {
		if ok[i] {
			r := find(i)
			groups[r] = append(groups[r], paths[i])
		}
	}
	var out []SimilarGroup
	for _, g := range groups {
		if len(g) < 2 {
			continue
		}
		SortBySize(g)
		out = append(out, SimilarGroup{Items: g})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Extra() > out[j].Extra() })
	return out
}
