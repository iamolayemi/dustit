package cmd

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/finder"
	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/targets"
	"github.com/iamolayemi/macclean/internal/ui"
)

func newScanCmd() *cobra.Command {
	var verbose, asJSON bool
	c := &cobra.Command{
		Use:   "scan",
		Short: "Show how much each cleanup category would free (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stop := ui.Spin("Scanning… (read-only, nothing is deleted)")
			results := targets.ScanAll(targets.All())
			insights := finder.Insights(ui.Human)
			stop()
			if asJSON {
				return writeJSON(scanJSON(results, insights, fsx.FreeSpace(), verbose))
			}

			printResults(results, verbose)
			if len(insights) > 0 {
				fmt.Println(ui.Bold.Render("Also worth knowing:"))
				for _, in := range insights {
					fmt.Println("  • " + in.Text)
					fmt.Println("    " + ui.Dim.Render(in.Hint))
				}
				fmt.Println()
			}
			fmt.Printf("Free space: %s\n", ui.Bold.Render(ui.Human(fsx.FreeSpace())))
			ui.Hint("Next: `macclean clean` (safe categories) or `macclean clean --only old-files,duplicates`")
			return nil
		},
	}
	c.Flags().BoolVarP(&verbose, "verbose", "v", false, "show the largest items in each category (with --json: include every item)")
	c.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return c
}

func printResults(results []targets.Result, verbose bool) {
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		size, items := ui.Human(r.Bytes), ""
		switch {
		case r.Unavailable != "":
			size = "—"
		case !r.SizeKnown():
			size = "?"
		case r.T.UpTo && r.Bytes > 0:
			size = "≤" + size
		}
		if len(r.T.CleanCmd) == 0 && r.Unavailable == "" {
			items = strconv.Itoa(len(r.Items))
		}
		name := r.T.Name
		if r.Unavailable != "" {
			name += " (" + r.Unavailable + ")"
		}
		rows = append(rows, []string{r.T.ID, size, items, r.T.Risk.String(), name})
	}

	fmt.Println(ui.Table([]string{"CATEGORY", "SIZE", "ITEMS", "TYPE", "WHAT"}, rows, func(row, col int) lipgloss.Style {
		r := results[row]
		s := lipgloss.NewStyle()
		if col == 1 || col == 2 {
			s = s.Align(lipgloss.Right)
		}
		switch {
		case !r.HasWork():
			return s.Faint(true)
		case col == 3 && r.T.Risk == targets.Safe:
			return s.Inherit(ui.Green)
		case col == 3:
			return s.Inherit(ui.Yellow)
		case col == 1 && r.Bytes >= 1e9:
			return s.Bold(true)
		}
		return s
	}))

	if verbose {
		for _, r := range results {
			if len(r.Items) == 0 {
				continue
			}
			fmt.Printf("\n%s %s\n", ui.Bold.Render(r.T.ID), ui.Dim.Render(ui.Human(r.Bytes)))
			for i, it := range r.Items {
				if i == 8 {
					ui.Hint("      … and %d more", len(r.Items)-8)
					break
				}
				note := ""
				if it.Note != "" {
					note = "  " + ui.Dim.Render("("+it.Note+")")
				}
				fmt.Printf("  %9s  %s%s\n", ui.Human(it.Size), fsx.Pretty(it.Path), note)
			}
		}
	}

	safe, review, denied := targets.Totals(results)
	fmt.Printf("\n  %s %s    %s %s\n\n",
		ui.Green.Render("Safe to reclaim:"), ui.Bold.Render(ui.Human(safe)),
		ui.Yellow.Render("Needs review:"), ui.Human(review))
	if denied > 0 {
		ui.Hint(ui.FDATip + "\n")
	}
}
