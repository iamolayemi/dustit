package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/dustit/internal/finder"
	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/ui"
)

func newSimilarCmd() *cobra.Command {
	var threshold, n int
	var del bool
	minSize := newSize("50KB")
	c := &cobra.Command{
		Use:   "similar [path]",
		Short: "Find similar-looking photos (resized copies, edits, bursts), not just exact duplicates",
		Long: `Compares loose images (JPEG, PNG, GIF, HEIC, TIFF, WebP) by how they look, using a perceptual
hash, so resized copies, re-saved JPEGs, light edits and burst shots are grouped together.
Looks in Pictures, Desktop, Downloads, Documents and your project folders by default; files
inside the Photos library are never touched. --threshold sets how alike images must be
(0 = identical-looking, 10 = loosely similar; default 6).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			roots := finder.UserFileRoots(true)
			if len(args) > 0 {
				roots = []string{fsx.Expand(args[0])}
			}
			stop := ui.Spin("Comparing images in " + prettyList(roots) + " … (a few minutes for big folders)")
			groups := finder.SimilarImages(roots, threshold, minSize.n)
			stop()
			if len(groups) == 0 {
				fmt.Println("No similar images found.")
				return nil
			}
			var extra int64
			for _, g := range groups {
				extra += g.Extra()
			}
			shown := groups
			if len(shown) > n {
				shown = shown[:n]
			}
			fmt.Println()
			for i, g := range shown {
				fmt.Printf("  %s  %d similar images\n", ui.Bold.Render(strconv.Itoa(i+1)+"."), len(g.Items))
				for j, it := range g.Items {
					mark := ""
					if j == 0 {
						mark = ui.Green.Render("  ← largest (keep)")
					}
					fmt.Printf("      %s %9s  %s%s\n", ui.Dim.Render(fmt.Sprintf("[%d]", j+1)), ui.Human(it.Size), fsx.Pretty(it.Path), mark)
				}
			}
			fmt.Printf("\n  %d groups; up to %s in extra copies.\n\n", len(groups), ui.Bold.Render(ui.Human(extra)))
			if !del {
				ui.Hint("Look before deleting: similar isn't identical. Re-run with --delete to choose per group.")
				return nil
			}
			var freed int64
			for i, g := range shown {
				a := ui.Ask(fmt.Sprintf("Group %d: keep which? [1-%d, Enter = 1 (largest), a = keep all, q = quit] ", i+1, len(g.Items)))
				if a == "q" {
					break
				}
				if a == "a" {
					continue
				}
				keep := 1
				if a != "" {
					k, err := strconv.Atoi(a)
					if err != nil || k < 1 || k > len(g.Items) {
						continue
					}
					keep = k
				}
				for j, it := range g.Items {
					if j == keep-1 {
						continue
					}
					if err := fsx.RemoveSized(it.Path, true, it.Size); err != nil {
						ui.Err("  %v", err)
						continue
					}
					freed += it.Size
				}
			}
			fmt.Printf("\nMoved %s to the Trash. Undo with: dustit undo\n", ui.Bold.Render(ui.Human(freed)))
			return nil
		},
	}
	f := c.Flags()
	f.IntVar(&threshold, "threshold", 6, "how alike images must be: 0 (identical-looking) to 10 (loosely similar)")
	f.Var(minSize, "min", "ignore images smaller than this")
	f.IntVarP(&n, "number", "n", 25, "how many groups to show")
	f.BoolVar(&del, "delete", false, "choose which image to keep in each group; the rest go to the Trash")
	return c
}
