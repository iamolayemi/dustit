package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/finder"
	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/ui"
)

func newStorageCmd() *cobra.Command {
	var apps, asJSON bool
	c := &cobra.Command{
		Use:   "storage",
		Short: "Where your space went, in plain groups (Downloads, Photos, Mail, backups, app data…)",
		Long: `Splits the disk into plain groups — Applications, Documents, Downloads, Photos, Mail,
Messages, iPhone backups, app data, developer files, caches, Trash, and macOS & system data —
with the command that frees each one. --apps shows which app owns each big app-data folder,
and flags folders whose app is no longer installed.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			stop := ui.Spin("Measuring your disk… (read-only, about a minute)")
			groups := finder.Storage()
			var owned []finder.OwnedFolder
			if apps {
				owned = finder.AppDataOwners(20)
			}
			stop()
			if asJSON {
				return writeJSON(map[string]any{"groups": groups, "app_data": owned})
			}
			free, total := fsx.DiskTotals()
			fmt.Printf("\n%s used of %s · %s free\n\n", ui.Bold.Render(ui.Human(total-free)), ui.Human(total), ui.Green.Render(ui.Human(free)))
			mx := groups[0].Size
			for _, g := range groups {
				if g.Size < 1e6 {
					continue
				}
				fmt.Printf("  %9s  %s  %-36s %s\n", ui.Human(g.Size), ui.Bar(g.Size, mx, 20), g.Name, ui.Dim.Render(g.Hint))
			}
			if !fsx.HasFullDiskAccess() {
				fmt.Println()
				ui.Hint("Mail, Messages and other apps' private data can't be measured without Full Disk Access,\nso they're counted under \"macOS, system data & other volumes\".\n" + ui.FDATip)
			}
			if apps {
				fmt.Println("\n" + ui.Bold.Render("Biggest app data folders"))
				for _, o := range owned {
					owner := o.Owner
					if !o.Installed {
						owner = ui.Yellow.Render(o.Owner + " — no matching app (a command-line tool, or a removed app)")
					}
					fmt.Printf("  %9s  %-40s %s\n", ui.Human(o.Size), owner, ui.Dim.Render(fsx.Pretty(o.Path)))
				}
			} else {
				ui.Hint("\nSee which app owns each data folder: macclean storage --apps")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&apps, "apps", false, "list the biggest app-data folders and the app that owns each")
	c.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return c
}
