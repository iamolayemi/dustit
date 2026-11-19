package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/dustit/internal/finder"
	"github.com/iamolayemi/dustit/internal/ui"
)

const (
	paneStorage = "x-apple.systempreferences:com.apple.Storage-Settings.extension"
	paneFDA     = "x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Privacy_AllFiles"
)

func newAttachmentsCmd() *cobra.Command {
	var open, asJSON bool
	c := &cobra.Command{
		Use:   "attachments",
		Short: "How much space Messages and Mail attachments take, by year (read-only)",
		Long: `Measures the attachments Messages and Mail keep on this Mac, by year, with the largest
files. It never deletes them: removing those files directly breaks the apps' databases.
Free the space with Apple's own tools instead, which DustIt points you to:

  Messages: System Settings → General → Storage → Messages → Review Large Attachments,
            or Messages → Settings → General → Keep messages: 1 Year
  Mail:     delete old messages with big attachments, then Mailbox → Erase Deleted Items

Reading these folders needs Full Disk Access for your terminal.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if open {
				return exec.Command("open", paneStorage).Run()
			}
			stop := ui.Spin("Measuring attachments…")
			report := finder.AttachmentReport(8)
			stop()
			if asJSON {
				return writeJSON(report)
			}
			anyUnreadable := false
			for _, a := range report {
				fmt.Printf("\n%s", ui.Bold.Render(a.App))
				if !a.Readable {
					anyUnreadable = true
					fmt.Println(ui.Dim.Render("  — can't read without Full Disk Access"))
					continue
				}
				fmt.Printf("  %s in %d files\n", ui.Human(a.Total), a.Files)
				var years []int
				for y := range a.ByYear {
					years = append(years, y)
				}
				sort.Ints(years)
				for _, y := range years {
					fmt.Printf("  %d  %9s  %s\n", y, ui.Human(a.ByYear[y]), ui.Bar(a.ByYear[y], a.Total, 20))
				}
				if len(a.Largest) > 0 {
					fmt.Println(ui.Dim.Render("  Largest:"))
					for _, it := range a.Largest {
						fmt.Printf("  %9s  %s  %s\n", ui.Human(it.Size), ui.Dim.Render(it.Age.Format("Jan 2006")), filepath.Base(it.Path))
					}
				}
			}
			fmt.Println()
			if anyUnreadable {
				ui.Hint("Give your terminal Full Disk Access to include them: open \"" + paneFDA + "\"")
			}
			ui.Hint("Free the space with Apple's tools (safe for the apps' databases): dustit attachments --open")
			ui.Hint("Messages: Review Large Attachments, or keep messages for 1 year. Mail: delete big old messages, then Erase Deleted Items.")
			return nil
		},
	}
	c.Flags().BoolVar(&open, "open", false, "open System Settings → General → Storage")
	c.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return c
}
