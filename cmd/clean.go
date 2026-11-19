package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/targets"
	"github.com/iamolayemi/macclean/internal/ui"
)

func newCleanCmd() *cobra.Command {
	var (
		only               []string
		all, yes, dry, tsh bool
	)
	c := &cobra.Command{
		Use:   "clean",
		Short: "Clean categories (safe ones by default; asks before each)",
		Example: `  macclean clean --dry-run
  macclean clean
  macclean clean --only npm-cache,stale-deps
  macclean clean --all`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sel, err := targets.Select(only, all)
			if err != nil {
				return err
			}
			stop := ui.Spin("Scanning…")
			results := targets.ScanAll(sel)
			stop()
			printResults(results, false)
			if dry {
				ui.Warn("Dry run — nothing will be removed.\n")
			}

			before := fsx.FreeSpace()
			var total int64
			trashed := false
			for _, r := range results {
				if !r.HasWork() {
					continue
				}
				size := ui.Human(r.Bytes)
				switch {
				case !r.SizeKnown():
					size = "size unknown"
				case r.T.UpTo:
					size = "up to " + size
				}
				q := fmt.Sprintf("Clean %s (%s)?", ui.Bold.Render(r.T.Name), size)
				if r.T.Risk == targets.Review {
					q = ui.Yellow.Render("[review] ") + q + "\n  " + ui.Dim.Render(r.T.Desc) + "\n "
				}
				if dry {
					fmt.Printf("%s (%s)\n", ui.Bold.Render(r.T.Name), size)
				} else if !yes && !ui.Confirm(q) {
					continue
				}

				freed, failed := targets.Clean(r, targets.CleanOptions{
					DryRun: dry, Trash: tsh,
					Log: func(line string) { fmt.Println(ui.Dim.Render("  " + line)) },
				})
				total += freed
				if (tsh || r.T.ToTrash) && len(r.Items) > 0 {
					trashed = true
				}
				msg := ui.Green.Render("  ✓ ") + ui.Human(freed)
				if dry {
					msg = ui.Dim.Render("  would free ~" + ui.Human(freed))
				}
				if failed > 0 {
					msg += ui.Yellow.Render(fmt.Sprintf("  (%d items skipped — in use or protected)", failed))
				}
				fmt.Println(msg)
			}

			fmt.Println()
			if dry {
				fmt.Printf("Would free about %s.\n", ui.Bold.Render(ui.Human(total)))
				return nil
			}
			after := fsx.FreeSpace()
			fmt.Printf("Freed about %s. Free space: %s → %s\n", ui.Bold.Render(ui.Human(total)), ui.Human(before), ui.Bold.Render(ui.Human(after)))
			if trashed {
				ui.Hint("Some items were moved to Trash. Empty it with: macclean clean --only trash")
			}
			if after-before < total/2 && total > 1e9 {
				ui.Hint("macOS can take a moment to report freed space; Time Machine local snapshots can also hold it\n(list them with: tmutil listlocalsnapshots /).")
			}
			return nil
		},
	}
	f := c.Flags()
	f.StringSliceVar(&only, "only", nil, "comma-separated category `ids` to clean (see: macclean list)")
	f.BoolVar(&all, "all", false, "include 'review' categories too")
	f.BoolVarP(&yes, "yes", "y", false, "don't ask before each category")
	f.BoolVarP(&dry, "dry-run", "n", false, "show what would be removed without removing anything")
	f.BoolVar(&tsh, "trash", false, "move items to Trash instead of deleting them")
	_ = c.RegisterFlagCompletionFunc("only", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return targets.IDs(), cobra.ShellCompDirectiveNoFileComp
	})
	return c
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all cleanup categories",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			for _, risk := range []targets.Risk{targets.Safe, targets.Review} {
				title := "Safe (cleaned by `macclean clean`):"
				if risk == targets.Review {
					title = "Review (only cleaned with --only <id> or --all):"
				}
				fmt.Println("\n" + ui.Bold.Render(title))
				for _, t := range targets.All() {
					if t.Risk == risk {
						fmt.Printf("  %-24s %s\n", t.ID, ui.Dim.Render(t.Desc))
					}
				}
			}
			fmt.Println()
		},
	}
}
