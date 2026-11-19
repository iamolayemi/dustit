package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/history"
	"github.com/iamolayemi/macclean/internal/ui"
)

func newHistoryCmd() *cobra.Command {
	var n int
	c := &cobra.Command{
		Use:   "history [run-id]",
		Short: "List what macclean removed, or the items of one run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := history.Load()
			if err != nil {
				return err
			}
			runs := history.Runs(records)
			if len(runs) == 0 {
				fmt.Println("Nothing has been removed yet.")
				return nil
			}
			if len(args) == 1 {
				r, err := history.Find(runs, args[0])
				if err != nil {
					return err
				}
				fmt.Printf("%s  %s  %s\n\n", ui.Bold.Render(r.ID), r.Command, ui.Dim.Render(ui.Ago(r.Time)))
				for _, rec := range r.Records {
					fmt.Printf("  %-9s %9s  %s %s\n", rec.Action, ui.Human(rec.Size), fsx.Pretty(rec.Path), ui.Dim.Render(rec.Category))
				}
				return nil
			}
			fmt.Println()
			for i, r := range runs {
				if i == n {
					ui.Hint("  … %d older runs (use -n)", len(runs)-n)
					break
				}
				what := fmt.Sprintf("%d trashed, %d deleted", r.Trashed, r.Deleted)
				if r.Restored > 0 {
					what = fmt.Sprintf("restored %d items", r.Restored)
				}
				status := ""
				switch {
				case r.Undone:
					status = ui.Dim.Render("(undone)")
				case r.Trashed > 0:
					status = ui.Green.Render("(undoable)")
				}
				fmt.Printf("  %s  %-14s %9s  %-24s %-32s %s\n", ui.Bold.Render(r.ID), ui.Ago(r.Time), ui.Human(r.Bytes), what, r.Command, status)
			}
			fmt.Println()
			ui.Hint("Details: macclean history <run-id>   Put trashed items back: macclean undo [run-id]")
			ui.Hint("Log file: %s", fsx.Pretty(history.File()))
			return nil
		},
	}
	c.Flags().IntVarP(&n, "number", "n", 15, "how many runs to show")
	return c
}

func newUndoCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "undo [run-id]",
		Short: "Put items a run moved to the Trash back where they were",
		Long: `Moves the items a run sent to the Trash back to their original location.
Without a run id it undoes the most recent run that still has items in the Trash.

Items that were deleted permanently (caches, dependency folders…) can't be restored;
they are rebuilt or re-downloaded by the tools that own them.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := history.Load()
			if err != nil {
				return err
			}
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			r, err := history.Find(history.Runs(records), id)
			if err != nil {
				return err
			}
			if r.Trashed == 0 {
				return fmt.Errorf("run %s didn't move anything to the Trash, so there's nothing to put back", r.ID)
			}
			fmt.Printf("Run %s (%s, %s): %d items in the Trash", r.ID, r.Command, ui.Ago(r.Time), r.Trashed)
			if r.Deleted > 0 {
				fmt.Printf(", %d deleted permanently (can't be restored)", r.Deleted)
			}
			fmt.Println()
			if !yes && !ui.Confirm("Put the trashed items back?") {
				return nil
			}
			res := history.Undo(r)
			fmt.Printf("%s Restored %d items (%s).\n", ui.Green.Render("✓"), res.Restored, ui.Human(res.Bytes))
			for _, p := range res.Missing {
				ui.Warn("  no longer in the Trash: %s", fsx.Pretty(p))
			}
			for _, p := range res.Occupied {
				ui.Warn("  something new is already at %s — left in the Trash", fsx.Pretty(p))
			}
			for _, e := range res.Errors {
				ui.Err("  %v", e)
			}
			return nil
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask for confirmation")
	return c
}
