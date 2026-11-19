package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/dustit/internal/privacy"
	"github.com/iamolayemi/dustit/internal/ui"
)

func newPrivacyCmd() *cobra.Command {
	var open string
	var asJSON bool
	c := &cobra.Command{
		Use:   "privacy",
		Short: "Read-only check of your privacy and security settings",
		Long: `Checks FileVault, the firewall, Gatekeeper, System Integrity Protection, automatic
updates, password after sleep, sharing services, AirDrop, Find My, automatic login, the guest
account and (with Full Disk Access) which apps have powerful permissions.

Nothing is changed. Open the right System Settings pane for any check with:
  dustit privacy --open <id>`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			checks := privacy.Run()
			if open != "" {
				for _, c := range checks {
					if c.ID == open {
						return exec.Command("open", c.Pane).Run()
					}
				}
				return fmt.Errorf("no check called %q (ids are shown on the right in `dustit privacy`)", open)
			}
			if asJSON {
				return writeJSON(checks)
			}
			fmt.Println()
			risks, notices := 0, 0
			for _, c := range checks {
				icon := ui.Green.Render("✓")
				switch c.Status {
				case privacy.Risk:
					icon = ui.Red.Render("✗")
					risks++
				case privacy.Notice:
					icon = ui.Yellow.Render("!")
					notices++
				case privacy.Unknown:
					icon = ui.Dim.Render("?")
				}
				fmt.Printf("%s %-30s %-48s %s\n", icon, c.Name, c.Summary, ui.Dim.Render(c.ID))
				if c.Advice != "" && c.Status != privacy.Good {
					fmt.Println("  " + ui.Dim.Render(c.Advice))
				}
			}
			fmt.Println()
			switch {
			case risks > 0:
				fmt.Printf("%d setting(s) weaken your protection. ", risks)
			case notices > 0:
				fmt.Printf("Nothing serious; %d thing(s) worth a look. ", notices)
			default:
				fmt.Print("Everything looks good. ")
			}
			ui.Hint("Open a setting: dustit privacy --open <id>")
			return nil
		},
	}
	c.Flags().StringVar(&open, "open", "", "open the System Settings pane for a check (e.g. firewall)")
	c.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return c
}
