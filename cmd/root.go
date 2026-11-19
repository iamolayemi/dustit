// Package cmd wires macclean's commands together with cobra.
package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
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
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor || os.Getenv("NO_COLOR") != "" {
				lipgloss.SetColorProfile(termenv.Ascii)
			}
			if runtime.GOOS != "darwin" {
				fmt.Fprintln(os.Stderr, "warning: macclean is built for macOS; most paths won't exist here")
			}
		},
	}
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colours")

	root.AddGroup(
		&cobra.Group{ID: "clean", Title: "Clean up:"},
		&cobra.Group{ID: "find", Title: "Find what uses space:"},
		&cobra.Group{ID: "speed", Title: "Speed:"},
	)
	doctorCmd := newDoctorCmd()
	doctorCmd.GroupID = "speed"
	root.AddCommand(doctorCmd)
	for _, c := range []*cobra.Command{newScanCmd(), newCleanCmd(), newListCmd(), newDockerCmd()} {
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
