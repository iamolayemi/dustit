package finder

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func scene(w, h int, brighten uint8, flip bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			xx := x
			if flip {
				xx = w - 1 - x
			}
			v := uint8((xx * 255 / w) / 2)
			if (xx/(w/6)+y/(h/4))%2 == 0 {
				v += 90
			}
			img.Set(x, y, color.RGBA{v + brighten, v / 2, 255 - v, 255})
		}
	}
	return img
}

func TestSimilarImages(t *testing.T) {
	root := t.TempDir()
	save := func(name string, img image.Image) {
		f, err := os.Create(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if filepath.Ext(name) == ".png" {
			_ = png.Encode(f, img)
		} else {
			_ = jpeg.Encode(f, img, &jpeg.Options{Quality: 70})
		}
	}
	save("original.png", scene(640, 480, 0, false))
	save("smaller copy.jpg", scene(320, 240, 6, false)) // resized, re-encoded, a bit brighter
	save("different.png", scene(640, 480, 0, true))     // mirrored: a different picture

	groups := SimilarImages([]string{root}, 8, 0)
	if len(groups) != 1 || len(groups[0].Items) != 2 {
		t.Fatalf("want one group of two, got %+v", groups)
	}
	for _, it := range groups[0].Items {
		if filepath.Base(it.Path) == "different.png" {
			t.Error("the mirrored image must not be grouped")
		}
	}
}
