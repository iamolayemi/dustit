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
	c.AddCommand(on, off)
	return c
}

func showSchedule() {
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
