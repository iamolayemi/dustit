package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/schedule"
	"github.com/iamolayemi/dustit/internal/timeline"
	"github.com/iamolayemi/dustit/internal/ui"
	"github.com/iamolayemi/dustit/internal/watch"
)

// parseSince accepts "7d", "2w", "12h", "30m".
func parseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil && strings.HasSuffix(s, "d") {
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if n, err := strconv.Atoi(strings.TrimSuffix(s, "w")); err == nil && strings.HasSuffix(s, "w") {
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func signed(n int64) string {
	if n >= 0 {
		return "+" + ui.Human(n)
	}
	return "−" + ui.Human(-n)
}

func newSnapshotCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot",
		Short: "Record how disk space is used right now (for dustit growth)",
		Long: `Measures the storage groups and every folder over 50 MB and adds them to the timeline in
~/Library/Application Support/dustit. "dustit schedule watch on" takes one every day.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			stop := ui.Spin("Measuring your disk… (about a minute)")
			s := timeline.Take()
			stop()
			if err := timeline.Save(s); err != nil {
				return err
			}
			all, _ := timeline.Load()
			fmt.Printf("%s Snapshot saved (%d in the timeline). See changes with: dustit growth\n", ui.Green.Render("✓"), len(all))
			return nil
		},
	}
}

func newGrowthCmd() *cobra.Command {
	var since string
	var asJSON bool
	c := &cobra.Command{
		Use:   "growth",
		Short: "What grew and what shrank since a past snapshot (default: a week)",
		Long: `Compares the disk now with the snapshot closest to --since ago (7d, 2w, 24h…) and shows
which groups and folders grew or shrank. It needs at least one earlier snapshot: run
"dustit snapshot" now, or "dustit schedule watch on" to take one every day.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			d, err := parseSince(since)
			if err != nil {
				return fmt.Errorf("--since %q: use 7d, 2w, 24h…", since)
			}
			all, err := timeline.Load()
			if err != nil {
				return err
			}
			if len(all) == 0 {
				fmt.Println("No snapshots yet. Take one now with `dustit snapshot` (or turn on daily ones: dustit schedule watch on),")
				fmt.Println("then come back later to see what grew.")
				return nil
			}
			stop := ui.Spin("Measuring your disk now…")
			now := timeline.Take()
			stop()
			timeline.SaveIfStale(now, 6*time.Hour)
			then, _ := timeline.Before(all, time.Now().Add(-d))
			groups, folders := timeline.Diff(then, now, 200e6)
			if asJSON {
				return writeJSON(map[string]any{"since": then.Time, "free_before": then.Free, "free_now": now.Free, "groups": groups, "folders": folders})
			}
			fmt.Printf("\nSince %s (%s ago): free space %s → %s (%s)\n\n",
				then.Time.Format("Mon 2 Jan 15:04"), ui.Since(then.Time), ui.Human(then.Free), ui.Human(now.Free), signed(now.Free-then.Free))
			if len(groups)+len(folders) == 0 {
				fmt.Println("Nothing changed by more than 200 MB.")
				return nil
			}
			print := func(title string, cs []timeline.Change) {
				if len(cs) == 0 {
					return
				}
				fmt.Println(ui.Bold.Render(title))
				for i, c := range cs {
					if i == 12 {
						ui.Hint("  … and %d more", len(cs)-12)
						break
					}
					val := signed(c.Delta)
					if c.Delta > 0 {
						val = ui.Yellow.Render(fmt.Sprintf("%10s", val))
					} else {
						val = ui.Green.Render(fmt.Sprintf("%10s", val))
					}
					fmt.Printf("  %s  %-40s %s\n", val, c.Name, ui.Dim.Render(ui.Human(c.Before)+" → "+ui.Human(c.After)))
				}
				fmt.Println()
			}
			print("Groups", groups)
			print("Folders", folders)
			ui.Hint("Drill in with: dustit explore <folder>")
			return nil
		},
	}
	c.Flags().StringVar(&since, "since", "7d", "compare with the snapshot from this long ago (7d, 2w, 24h…)")
	c.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return c
}

func newWatchCmd() *cobra.Command {
	var notify bool
	c := &cobra.Command{
		Use:   "watch",
		Short: "Check for runaway processes and sudden disk growth (run it in the background)",
		Long: `One check: processes using over 80% CPU for 30+ minutes, free space dropping 10 GB within
90 minutes, and (once a day) a timeline snapshot that flags folders growing 5 GB in a day.
It remembers state between runs, so it's meant to run every 15 minutes:

  dustit schedule watch on       run it in the background with notifications
  dustit schedule watch off`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			alerts := watch.Check(time.Now(), fsx.FreeSpace(), ui.Human)
			if len(alerts) == 0 {
				fmt.Println(ui.Green.Render("✓") + " Nothing unusual.")
				return nil
			}
			for _, a := range alerts {
				fmt.Printf("%s %s — %s\n", ui.Yellow.Render("!"), ui.Bold.Render(a.Title), a.Body)
				if notify && watch.ShouldNotify(a, time.Now()) {
					sendNotification(a.Title, a.Body)
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&notify, "notify", false, "show macOS notifications (at most one per issue every 6 hours)")
	return c
}

func newScheduleWatchCmd() *cobra.Command {
	var every time.Duration
	var dry bool
	w := &cobra.Command{
		Use:   "watch",
		Short: "Watch for runaway processes and sudden disk growth in the background",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if e, ok := schedule.WatchInstalled(); ok {
				fmt.Printf("Background watch: every %s\n", e)
			} else {
				fmt.Println("Background watch is off. Turn it on with: dustit schedule watch on")
			}
			return nil
		},
	}
	on := &cobra.Command{
		Use:   "on",
		Short: "Turn on the background watch",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			bin, err := schedule.Binary()
			if err != nil {
				return err
			}
			if dry {
				p, err := schedule.WatchPlist(bin, every)
				if err != nil {
					return err
				}
				fmt.Printf("Would write %s:\n\n%s", fsx.Pretty(schedule.WatchPlistPath()), p)
				return nil
			}
			if err := schedule.InstallWatch(bin, every); err != nil {
				return err
			}
			fmt.Printf("%s DustIt checks every %s for runaway processes and sudden disk growth, and takes a daily snapshot.\n", ui.Green.Render("✓"), every)
			return nil
		},
	}
	on.Flags().DurationVar(&every, "every", 15*time.Minute, "how often to check")
	on.Flags().BoolVarP(&dry, "dry-run", "n", false, "print the launchd agent instead of installing it")
	off := &cobra.Command{
		Use:   "off",
		Short: "Turn off the background watch",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if err := schedule.RemoveWatch(); err != nil {
				return err
			}
			fmt.Println(ui.Green.Render("✓") + " Background watch removed.")
			return nil
		},
	}
	w.AddCommand(on, off)
	return w
}
