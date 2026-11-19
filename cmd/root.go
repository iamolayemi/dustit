// Package cmd wires macclean's commands together with cobra.
package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/history"
)

// Set at build time by GoReleaser (-ldflags "-X github.com/iamolayemi/macclean/cmd.version=…").
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newRootCmd() *cobra.Command {
	var noColor bool
	root := &cobra.Command{
		Use:   "macclean",
		Short: "Deep-clean your Mac, free disk space and find what slows it down",
		Long: `macclean finds caches, logs, stale project dependencies, old and duplicate files,
leftovers from deleted apps and unused Docker data, and removes them when you say so.

Nothing is deleted without asking unless you pass --yes. Start with:

  macclean scan`,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			history.Begin(cmd.CommandPath())
			// A broken config must not lock you out of fixing it.
			if err := applyConfig(); err != nil && !strings.HasPrefix(cmd.CommandPath(), "macclean config") {
				return fmt.Errorf("%w\nFix it with: macclean config edit", err)
			}
			if noColor || os.Getenv("NO_COLOR") != "" {
				lipgloss.SetColorProfile(termenv.Ascii)
			}
			if runtime.GOOS != "darwin" {
				fmt.Fprintln(os.Stderr, "warning: macclean is built for macOS; most paths won't exist here")
			}
			return nil
		},
	}
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colours")

	root.AddGroup(
		&cobra.Group{ID: "clean", Title: "Clean up:"},
		&cobra.Group{ID: "find", Title: "Find what uses space:"},
		&cobra.Group{ID: "speed", Title: "Speed:"},
	)
	for _, c := range []*cobra.Command{newConfigCmd(), newScheduleCmd()} {
		c.GroupID = "clean"
		root.AddCommand(c)
	}
	root.AddCommand(newGenCmd(root))
	doctorCmd := newDoctorCmd()
	doctorCmd.GroupID = "speed"
	root.AddCommand(doctorCmd)
	for _, c := range []*cobra.Command{newScanCmd(), newCleanCmd(), newUICmd(), newListCmd(), newDockerCmd(), newHistoryCmd(), newUndoCmd()} {
		c.GroupID = "clean"
		root.AddCommand(c)
	}
	for _, c := range []*cobra.Command{newTopCmd(), newLargeCmd(), newOldCmd(), newProjectsCmd(),
		newDupesCmd(), newAppsCmd(), newLeftoversCmd()} {
		c.GroupID = "find"
		root.AddCommand(c)
	}
	return root
}

// Execute runs the CLI.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
