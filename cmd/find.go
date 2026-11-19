package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/finder"
	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/ui"
)

func newTopCmd() *cobra.Command {
	var n int
	c := &cobra.Command{
		Use:   "top [path]",
		Short: "Show which folders use the most space (default: ~)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := pathArg(args, fsx.Home)
			stop := ui.Spin("Measuring " + fsx.Pretty(root) + " …")
			items, denied, err := finder.Children(root)
			stop()
			if err != nil {
				return err
			}
			fmt.Println()
			for i, it := range items {
				if i >= n || it.Size == 0 {
					break
				}
				fmt.Printf("  %9s  %s  %s\n", ui.Human(it.Size), ui.Bar(it.Size, items[0].Size, 24), fsx.Pretty(it.Path))
			}
			fmt.Printf("\n  %9s  total in %s\n\n", ui.Bold.Render(ui.Human(finder.Total(items))), fsx.Pretty(root))
			if denied > 0 {
				ui.Hint(ui.FDATip)
			}
			ui.Hint("Drill down with: macclean top <folder>")
			return nil
		},
	}
	c.Flags().IntVarP(&n, "number", "n", 20, "how many entries to show")
	return c
}

func newLargeCmd() *cobra.Command {
	var (
		n         int
		del, perm bool
	)
	minSize := newSize("500MB")
	c := &cobra.Command{
		Use:   "large [path]",
		Short: "Find large files (default: ~)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := pathArg(args, fsx.Home)
			stop := ui.Spin(fmt.Sprintf("Looking for files ≥ %s in %s … (can take a minute)", ui.Human(minSize.n), fsx.Pretty(root)))
			found := finder.LargeFiles(root, minSize.n)
			stop()
			if len(found) > n {
				found = found[:n]
			}
			if len(found) == 0 {
				fmt.Println("No files that large.")
				return nil
			}
			printItems(found, func(it finder.Item) string { return ui.Ago(it.Age) })
			fmt.Printf("  %s in %d files\n\n", ui.Bold.Render(ui.Human(finder.Total(found))), len(found))
			if del {
				reviewEach(found, !perm)
			} else {
				ui.Hint("Re-run with --delete to pick files to move to Trash.")
			}
			return nil
		},
	}
	f := c.Flags()
	f.Var(minSize, "min", "minimum file size")
	f.IntVarP(&n, "number", "n", 30, "how many files to show")
	f.BoolVar(&del, "delete", false, "go through the list and choose files to remove")
	f.BoolVar(&perm, "permanent", false, "with --delete: delete instead of moving to Trash")
	return c
}

func newOldCmd() *cobra.Command {
	var (
		days            int
		del, each, perm bool
	)
	minSize := newSize("0")
	c := &cobra.Command{
		Use:   "old [path]",
		Short: "Find old files and folders (default: ~/Downloads)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := pathArg(args, fsx.H("Downloads"))
			found, err := finder.OldEntries(root, days, minSize.n)
			if err != nil {
				return err
			}
			if len(found) == 0 {
				fmt.Printf("Nothing in %s older than %d days.\n", fsx.Pretty(root), days)
				return nil
			}
			printItems(found, func(it finder.Item) string { return ui.Ago(it.Age) })
			fmt.Printf("  %s in %d items older than %d days\n\n", ui.Bold.Render(ui.Human(finder.Total(found))), len(found), days)
			switch {
			case !del:
				ui.Hint("Re-run with --delete to move them to Trash (add --each to choose one by one).")
			case each:
				reviewEach(found, !perm)
			default:
				offerRemoval(found, !perm, "items")
			}
			return nil
		},
	}
	f := c.Flags()
	f.IntVar(&days, "days", 180, "show items not changed in this many days")
	f.Var(minSize, "min", "ignore items smaller than this")
	f.BoolVar(&del, "delete", false, "remove them (asks first)")
	f.BoolVar(&each, "each", false, "with --delete: ask about each item")
	f.BoolVar(&perm, "permanent", false, "with --delete: delete instead of moving to Trash")
	return c
}

func newProjectsCmd() *cobra.Command {
	var (
		days  int
		clean bool
	)
	c := &cobra.Command{
		Use:   "projects [path]",
		Short: "Find node_modules, vendor, Pods, target, .venv… in idle projects",
		Long: `Finds dependency and build folders in projects you haven't touched for a while.
A folder only counts when its project file sits next to it (node_modules next to
package.json, vendor next to composer.json, …). A project is idle when neither git
nor any source file changed in --days days.

Searches ~/Projects, ~/Developer, ~/code, ~/src, ~/dev, ~/workspace, ~/Sites,
~/Documents and ~/Desktop by default.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			roots := finder.ProjectRoots()
			if len(args) > 0 {
				roots = []string{fsx.Expand(args[0])}
			} else if len(roots) == 0 {
				roots = []string{fsx.Home}
			}
			stop := ui.Spin("Searching " + prettyList(roots) + " …")
			found := finder.StaleDeps(roots, days)
			stop()
			if len(found) == 0 {
				fmt.Printf("No dependency/build folders in projects untouched for %d+ days.\n", days)
				return nil
			}
			printItems(found, func(it finder.Item) string { return "idle " + ui.Since(it.Age) })
			fmt.Printf("  %s in %d folders (recreate with npm/composer/pod/cargo install etc.)\n\n",
				ui.Bold.Render(ui.Human(finder.Total(found))), len(found))
			if clean {
				offerRemoval(found, false, "folders")
			} else {
				ui.Hint("Re-run with --clean to remove them.")
			}
			return nil
		},
	}
	c.Flags().IntVar(&days, "days", 30, "only projects untouched for this many days (0 = all)")
	c.Flags().BoolVar(&clean, "clean", false, "remove the found folders (asks first)")
	return c
}

func newDupesCmd() *cobra.Command {
	var (
		n   int
		del bool
	)
	minSize := newSize("1MB")
	c := &cobra.Command{
		Use:   "dupes [path]",
		Short: "Find duplicate files (default: personal and project folders)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			roots := finder.UserFileRoots(true)
			if len(args) > 0 {
				roots = []string{fsx.Expand(args[0])}
			}
			stop := ui.Spin(fmt.Sprintf("Looking for duplicates ≥ %s in %s …", ui.Human(minSize.n), prettyList(roots)))
			groups := finder.Dupes(roots, minSize.n)
			stop()
			if len(groups) == 0 {
				fmt.Println("No duplicates found.")
				return nil
			}
			var wasted int64
			for _, g := range groups {
				wasted += g.Wasted()
			}
			if len(groups) > n {
				groups = groups[:n]
			}
			fmt.Println()
			for i, g := range groups {
				k := g.Keeper()
				fmt.Printf("  %s  %d copies × %s\n", ui.Bold.Render(strconv.Itoa(i+1)+"."), len(g.Paths), ui.Human(g.Size))
				for j, p := range g.Paths {
					mark := ""
					if j == k {
						mark = ui.Green.Render("  ← oldest")
					}
					fmt.Printf("      %s %s%s\n", ui.Dim.Render(fmt.Sprintf("[%d]", j+1)), fsx.Pretty(p), mark)
				}
			}
			fmt.Printf("\n  Up to %s could be freed by removing extra copies.\n", ui.Bold.Render(ui.Human(wasted)))
			ui.Hint("  Copies made in Finder on APFS can be clones that share space, so the real saving may be less.\n")
			if !del {
				ui.Hint("Re-run with --delete to choose which copy to keep in each group.")
				return nil
			}
			var freed int64
			for i, g := range groups {
				a := ui.Ask(fmt.Sprintf("Group %d: keep which copy? [1-%d, Enter = oldest, s = skip, q = quit] ", i+1, len(g.Paths)))
				if a == "q" {
					break
				}
				if a == "s" {
					continue
				}
				keep := g.Keeper() + 1
				if a != "" {
					k, err := strconv.Atoi(a)
					if err != nil || k < 1 || k > len(g.Paths) {
						continue
					}
					keep = k
				}
				for j, p := range g.Paths {
					if j == keep-1 {
						continue
					}
					if err := fsx.Remove(p, true); err != nil {
						ui.Err("  %v", err)
						continue
					}
					freed += g.Size
				}
			}
			fmt.Printf("\nMoved %s to Trash.\n", ui.Bold.Render(ui.Human(freed)))
			return nil
		},
	}
	f := c.Flags()
	f.Var(minSize, "min", "ignore files smaller than this")
	f.IntVarP(&n, "number", "n", 25, "how many duplicate groups to show")
	f.BoolVar(&del, "delete", false, "for each group, choose which copy to keep; the rest go to Trash")
	return c
}

func newAppsCmd() *cobra.Command {
	var (
		days      int
		uninstall bool
	)
	c := &cobra.Command{
		Use:   "apps",
		Short: "List installed apps by last use and size",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stop := ui.Spin("Reading installed apps…")
			apps := finder.Apps(true)
			stop()
			cutoff := time.Now().AddDate(0, 0, -days)
			var shown []finder.App
			for _, a := range apps {
				if strings.HasPrefix(a.ID, "com.apple.") || (days > 0 && a.LastUsed.After(cutoff)) {
					continue
				}
				shown = append(shown, a)
			}
			sort.Slice(shown, func(i, j int) bool {
				if shown[i].LastUsed.Equal(shown[j].LastUsed) {
					return shown[i].Size > shown[j].Size
				}
				return shown[i].LastUsed.Before(shown[j].LastUsed)
			})
			var total int64
			fmt.Println()
			for _, a := range shown {
				used := "never opened"
				if !a.LastUsed.IsZero() {
					used = "used " + ui.Ago(a.LastUsed)
				}
				total += a.Size
				fmt.Printf("  %9s  %s %s\n", ui.Human(a.Size), ui.Dim.Render(fmt.Sprintf("%-22s", used)), a.Name)
			}
			fmt.Printf("\n  %s in %d apps (Apple apps hidden)\n\n", ui.Bold.Render(ui.Human(total)), len(shown))
			if !uninstall {
				ui.Hint("Re-run with --uninstall to pick apps to remove (add --days 180 to see only unused ones).")
				return nil
			}
			var freed int64
			for _, a := range shown {
				data := finder.AppData(a)
				var dataSize int64
				for _, d := range data {
					dataSize += fsx.DiskUsage(d).Bytes
				}
				ans := ui.Ask(fmt.Sprintf("Move %s to Trash (%s app + %s data)? [y/N/q] ", ui.Bold.Render(a.Name), ui.Human(a.Size), ui.Human(dataSize)))
				if ans == "q" {
					break
				}
				if ans != "y" && ans != "yes" {
					continue
				}
				if err := fsx.TrashApp(a.Path); err != nil {
					ui.Err("  %v", err)
					continue
				}
				freed += a.Size
				for _, d := range data {
					sz := fsx.DiskUsage(d).Bytes
					if err := fsx.Remove(d, true); err != nil {
						ui.Warn("  kept %s: %v", fsx.Pretty(d), err)
						continue
					}
					freed += sz
				}
			}
			fmt.Printf("\nMoved %s to Trash. Empty it with: macclean clean --only trash\n", ui.Bold.Render(ui.Human(freed)))
			return nil
		},
	}
	c.Flags().IntVar(&days, "days", 0, "only show apps not opened in this many days")
	c.Flags().BoolVar(&uninstall, "uninstall", false, "choose apps to move to Trash, together with their data")
	return c
}

func newLeftoversCmd() *cobra.Command {
	var clean bool
	c := &cobra.Command{
		Use:   "leftovers",
		Short: "Find data left behind by apps you've already deleted",
		Long: `Looks in ~/Library (Containers, Caches, Application Support, Saved Application State,
HTTPStorages, WebKit, Logs) for folders named after a bundle ID (com.vendor.app) whose app
is no longer installed. Anything from a vendor that still has an app installed is kept.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stop := ui.Spin("Collecting installed apps…")
			found, err := finder.Leftovers()
			stop()
			if err != nil {
				return err
			}
			if len(found) == 0 {
				fmt.Println("No leftovers from removed apps found.")
				return nil
			}
			printItems(found, func(it finder.Item) string { return ui.Ago(it.Age) })
			fmt.Printf("  %s in %d folders from apps that no longer seem to be installed\n", ui.Bold.Render(ui.Human(finder.Total(found))), len(found))
			ui.Hint("  (command-line tools sometimes use names like these too — check the list)\n")
			if clean {
				offerRemoval(found, true, "folders")
			} else {
				ui.Hint("Re-run with --clean to move them to Trash.")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&clean, "clean", false, "move the leftovers to Trash (asks first)")
	return c
}
