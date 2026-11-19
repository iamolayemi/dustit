package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/schedule"
	"github.com/iamolayemi/macclean/internal/targets"
	"github.com/iamolayemi/macclean/internal/ui"
)

func newScheduleCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "schedule",
		Short: "Clean safe categories automatically (daily or weekly), with a notification",
		Long: `Runs "macclean clean" on a schedule through a launchd agent and shows a macOS
notification with how much was freed. Only safe categories can run unattended; the
default set leaves out dependency folders, the Trash and caches of apps you may have open.

  macclean schedule                         show the current schedule
  macclean schedule on                      every Monday at 10:00
  macclean schedule on --daily --at 21:30
  macclean schedule on --day fri --only npm-cache,user-caches
  macclean schedule off`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { showSchedule(); return nil },
	}

	var daily, dry bool
	var day, at string
	var only []string
	on := &cobra.Command{
		Use:   "on",
		Short: "Turn on (or change) the schedule",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			t, err := time.Parse("15:04", at)
			if err != nil {
				return fmt.Errorf("--at %q: use HH:MM, e.g. 10:00", at)
			}
			weekday := -1
			if !daily {
				d, ok := schedule.Weekdays[strings.ToLower(day)[:min(3, len(day))]]
				if !ok {
					return fmt.Errorf("--day %q: use mon, tue, wed, thu, fri, sat or sun", day)
				}
				weekday = d
			}
			cats := schedule.DefaultCategories
			if len(only) > 0 {
				cats = only
			}
			sel, err := targets.Select(cats, false)
			if err != nil {
				return err
			}
			for _, tg := range sel {
				if tg.Risk != targets.Safe {
					return fmt.Errorf("%s is a review category — those never run unattended", tg.ID)
				}
			}
			bin, err := schedule.Binary()
			if err != nil {
				return err
			}
			spec := schedule.Spec{Binary: bin, Categories: cats, Weekday: weekday, Hour: t.Hour(), Minute: t.Minute(), LogFile: schedule.LogPath()}
			if dry {
				p, err := schedule.Plist(spec)
				if err != nil {
					return err
				}
				fmt.Printf("Would write %s:\n\n%s", fsx.Pretty(schedule.PlistPath()), p)
				return nil
			}
			if err := schedule.Install(spec); err != nil {
				return err
			}
			fmt.Printf("%s macclean will clean %d categories %s.\n", ui.Green.Render("✓"), len(cats), schedule.Describe(weekday, t.Hour(), t.Minute()))
			ui.Hint("Log: %s   Turn off: macclean schedule off", fsx.Pretty(schedule.LogPath()))
			return nil
		},
	}
	on.Flags().BoolVar(&daily, "daily", false, "run every day instead of weekly")
	on.Flags().StringVar(&day, "day", "mon", "weekday for weekly runs (mon … sun)")
	on.Flags().StringVar(&at, "at", "10:00", "time of day, 24-hour HH:MM")
	on.Flags().StringSliceVar(&only, "only", nil, "safe category ids to clean (default: a conservative safe set)")
	on.Flags().BoolVarP(&dry, "dry-run", "n", false, "print the launchd agent instead of installing it")

	off := &cobra.Command{
		Use:   "off",
		Short: "Turn the schedule off",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if !schedule.Current().Installed {
				return errors.New("no schedule is set")
			}
			if err := schedule.Remove(); err != nil {
				return err
			}
			fmt.Println(ui.Green.Render("✓") + " Schedule removed.")
			return nil
		},
	}
	c.AddCommand(on, off, newScheduleAlertCmd())
	return c
}

func newScheduleAlertCmd() *cobra.Command {
	var below string
	var every time.Duration
	var dry bool
	a := &cobra.Command{
		Use:   "alert",
		Short: "Get a notification when free space runs low (never deletes anything)",
		Long: `Checks free space in the background and shows a notification when it drops below
the threshold (at most twice a day). It never cleans anything by itself.

  macclean schedule alert on                     below 10%, checked every hour
  macclean schedule alert on --below 20GB --every 30m
  macclean schedule alert off`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if b, e, ok := schedule.AlertInstalled(); ok {
				fmt.Printf("Low-disk alert: below %s, checked every %s\n", b, e)
			} else {
				fmt.Println("No low-disk alert. Turn it on with: macclean schedule alert on")
			}
			return nil
		},
	}
	on := &cobra.Command{
		Use:   "on",
		Short: "Turn on (or change) the low-disk alert",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if _, err := schedule.Threshold(below, ui.ParseSize); err != nil {
				return err
			}
			bin, err := schedule.Binary()
			if err != nil {
				return err
			}
			spec := schedule.AlertSpec{Binary: bin, Below: below, Every: every}
			if dry {
				p, err := schedule.AlertPlist(spec)
				if err != nil {
					return err
				}
				fmt.Printf("Would write %s:\n\n%s", fsx.Pretty(schedule.AlertPlistPath()), p)
				return nil
			}
			if err := schedule.InstallAlert(spec); err != nil {
				return err
			}
			fmt.Printf("%s You'll get a notification when free space drops below %s (checked every %s).\n", ui.Green.Render("✓"), below, every)
			return nil
		},
	}
	on.Flags().StringVar(&below, "below", "10%", "threshold: a percentage (10%) or a size (20GB)")
	on.Flags().DurationVar(&every, "every", time.Hour, "how often to check")
	on.Flags().BoolVarP(&dry, "dry-run", "n", false, "print the launchd agent instead of installing it")
	off := &cobra.Command{
		Use:   "off",
		Short: "Turn the low-disk alert off",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if err := schedule.RemoveAlert(); err != nil {
				return err
			}
			fmt.Println(ui.Green.Render("✓") + " Low-disk alert removed.")
			return nil
		},
	}
	a.AddCommand(on, off)
	return a
}

func newAlertCmd() *cobra.Command {
	var below string
	var notify bool
	c := &cobra.Command{
		Use:   "alert",
		Short: "Check free space against a threshold (exit status 1 when low)",
		Long: `Prints whether free space is below the threshold and exits with status 1 when it is,
so it works in scripts. With --notify it also shows a macOS notification (at most twice a day).
"macclean schedule alert on" runs this in the background.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			low, err := schedule.Threshold(below, ui.ParseSize)
			if err != nil {
				return err
			}
			free, total := fsx.DiskTotals()
			pct := float64(free) / float64(total) * 100
			if !low(free, total) {
				fmt.Printf("%s %s free (%.0f%%) — above %s.\n", ui.Green.Render("✓"), ui.Human(free), pct, below)
				return nil
			}
			msg := fmt.Sprintf("Only %s free (%.0f%%). Run macclean scan to see what can go.", ui.Human(free), pct)
			fmt.Println(ui.Red.Render("✗ ") + msg)
			if notify && schedule.ShouldNotify(12*time.Hour) {
				sendNotification("Your Mac is running out of space", msg)
			}
			return errQuietExit
		},
	}
	c.Flags().StringVar(&below, "below", "10%", "threshold: a percentage (10%) or a size (20GB)")
	c.Flags().BoolVar(&notify, "notify", false, "show a macOS notification when space is low")
	return c
}

func showSchedule() {
	defer func() {
		if b, e, ok := schedule.AlertInstalled(); ok {
			fmt.Printf("Low-disk alert: below %s, checked every %s\n", b, e)
		}
	}()
	st := schedule.Current()
	if !st.Installed {
		fmt.Println("No schedule. Turn one on with: macclean schedule on")
		ui.Hint("Default: every Monday at 10:00, cleaning %s", strings.Join(schedule.DefaultCategories, ", "))
		return
	}
	state := ui.Green.Render("active")
	if !st.Loaded {
		state = ui.Yellow.Render("installed but not loaded (log out and in, or run: macclean schedule on)")
	}
	fmt.Printf("Cleaning %s — %s\n", schedule.Describe(st.Weekday, st.Hour, st.Minute), state)
	fmt.Printf("Categories: %s\n", strings.Join(st.Categories, ", "))
	ui.Hint("Log: %s", fsx.Pretty(schedule.LogPath()))
}
