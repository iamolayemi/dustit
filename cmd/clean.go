package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/targets"
	"github.com/iamolayemi/dustit/internal/ui"
)

func newCleanCmd() *cobra.Command {
	var (
		only               []string
		all, yes, dry, tsh bool
		notify             bool
	)
	c := &cobra.Command{
		Use:   "clean",
		Short: "Clean categories (safe ones by default; asks before each)",
		Example: `  dustit clean --dry-run
  dustit clean
  dustit clean --only npm-cache,stale-deps
  dustit clean --all`,
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
			if notify {
				sendNotification("dustit", fmt.Sprintf("Freed %s · %s free", ui.Human(total), ui.Human(after)))
			}
			fmt.Printf("Freed about %s. Free space: %s → %s\n", ui.Bold.Render(ui.Human(total)), ui.Human(before), ui.Bold.Render(ui.Human(after)))
			if trashed {
				ui.Hint("Some items were moved to Trash. Empty it with: dustit clean --only trash")
			}
			if after-before < total/2 && total > 1e9 {
				ui.Hint("macOS can take a moment to report freed space; Time Machine local snapshots can also hold it\n(list them with: tmutil listlocalsnapshots /).")
			}
			return nil
		},
	}
	f := c.Flags()
	f.StringSliceVar(&only, "only", nil, "comma-separated category `ids` to clean (see: dustit list)")
	f.BoolVar(&all, "all", false, "include 'review' categories too")
	f.BoolVarP(&yes, "yes", "y", false, "don't ask before each category")
	f.BoolVarP(&dry, "dry-run", "n", false, "show what would be removed without removing anything")
	f.BoolVar(&tsh, "trash", false, "move items to Trash instead of deleting them")
	f.BoolVar(&notify, "notify", false, "show a macOS notification when done (used by scheduled runs)")
	_ = c.RegisterFlagCompletionFunc("only", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return targets.IDs(), cobra.ShellCompDirectiveNoFileComp
	})
	return c
}

func newListCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List all cleanup categories",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if asJSON {
				type entry struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Description string `json:"description"`
					Risk        string `json:"risk"`
				}
				var out []entry
				for _, t := range targets.All() {
					out = append(out, entry{t.ID, t.Name, t.Desc, t.Risk.String()})
				}
				_ = writeJSON(out)
				return
			}
			for _, risk := range []targets.Risk{targets.Safe, targets.Review} {
				title := "Safe (cleaned by `dustit clean`):"
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
	c.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return c
}

// sendNotification shows a macOS notification; failures are ignored.
func sendNotification(title, msg string) {
	q := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) }
	_ = exec.Command("osascript", "-e", fmt.Sprintf(`display notification "%s" with title "%s"`, q(msg), q(title))).Run()
}
