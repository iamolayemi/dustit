package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/doctor"
	"github.com/iamolayemi/macclean/internal/ui"
)

func newDoctorCmd() *cobra.Command {
	var fix, all, asJSON bool
	var watch bool
	var every time.Duration
	c := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose why your Mac feels slow, with specific fixes",
		Long: `Checks CPU load and the processes behind it, memory pressure and swap, disk space,
thermal throttling, battery health, pending macOS updates, Docker's memory, uptime,
startup agents, Homebrew background services and Time Machine snapshots, then tells
you what to do about each problem. --watch keeps a live view of CPU and memory.

Checks only read. With --fix, macclean offers the few fixes that are safe to automate,
asking before each:
  • restart a Homebrew service that has been burning CPU for hours
  • unload and trash startup agents whose program no longer exists

It never quits apps or "cleans RAM": macOS manages memory itself, and purging it
only makes apps slower while they reload.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if watch {
				return watchDoctor(every)
			}
			stop := ui.Spin("Examining your Mac…")
			findings := doctor.Run(ui.Human)
			stop()
			if asJSON {
				return writeJSON(doctorJSON(findings))
			}

			problems, fixes := 0, 0
			fmt.Println()
			for _, f := range findings {
				if f.Severity >= doctor.Warn {
					problems++
				}
				fixes += len(f.Fixes)
				if f.Severity == doctor.OK && !all {
					fmt.Printf("%s %s %s\n", ui.Green.Render("✓"), ui.Bold.Render(f.Check+":"), ui.Dim.Render(f.Summary))
					continue
				}
				fmt.Printf("%s %s %s\n", icon(f.Severity), ui.Bold.Render(f.Check+":"), f.Summary)
				for _, d := range f.Details {
					fmt.Println("    " + ui.Dim.Render(d))
				}
				for _, a := range f.Advice {
					fmt.Println("    " + ui.Cyan.Render("→ ") + a)
				}
				fmt.Println()
			}

			switch {
			case problems == 0:
				fmt.Println(ui.Green.Render("\nNothing is slowing your Mac down right now."))
			case fixes > 0 && !fix:
				fmt.Printf("\n%d problem(s) found. %d can be fixed automatically: %s\n", problems, fixes, ui.Bold.Render("macclean doctor --fix"))
			default:
				fmt.Printf("\n%d problem(s) found — see the suggestions above.\n", problems)
			}
			if !fix || fixes == 0 {
				return nil
			}

			fmt.Println()
			for _, f := range findings {
				for _, x := range f.Fixes {
					if !ui.Confirm(x.Description + "?") {
						continue
					}
					if err := x.Run(); err != nil {
						ui.Err("  failed: %v", err)
					} else {
						fmt.Println(ui.Green.Render("  ✓ done"))
					}
				}
			}
			ui.Hint("\nRun `macclean doctor` again in a minute to see the effect.")
			return nil
		},
	}
	c.Flags().BoolVar(&fix, "fix", false, "offer the safe automatic fixes, asking before each")
	c.Flags().BoolVar(&all, "verbose", false, "show details for checks that passed too")
	c.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	c.Flags().BoolVarP(&watch, "watch", "w", false, "live view of CPU and memory until Ctrl-C")
	c.Flags().DurationVar(&every, "every", 3*time.Second, "with --watch: refresh interval")
	return c
}

// watchDoctor redraws the CPU and memory picture every interval.
func watchDoctor(every time.Duration) error {
	if every < time.Second {
		every = time.Second
	}
	for {
		findings := doctor.Quick(ui.Human)
		fmt.Print("\033[H\033[2J") // clear screen
		fmt.Printf("%s  %s\n\n", ui.Bold.Render("macclean doctor --watch"), ui.Dim.Render(time.Now().Format("15:04:05")+" · every "+every.String()+" · Ctrl-C to stop"))
		for _, f := range findings {
			fmt.Printf("%s %s %s\n", icon(f.Severity), ui.Bold.Render(f.Check+":"), f.Summary)
			if f.Check == "CPU load" || f.Check == "Memory" {
				for _, d := range f.Details {
					fmt.Println("    " + ui.Dim.Render(d))
				}
			}
			fmt.Println()
		}
		time.Sleep(every)
	}
}

func icon(s doctor.Severity) string {
	switch s {
	case doctor.Bad:
		return ui.Red.Render("✗")
	case doctor.Warn:
		return ui.Yellow.Render("!")
	case doctor.Info:
		return ui.Cyan.Render("i")
	}
	return ui.Green.Render("✓")
}
