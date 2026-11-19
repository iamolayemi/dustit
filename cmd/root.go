// Package cmd wires dustit's commands together with cobra.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/iamolayemi/dustit/internal/history"
)

// Set at build time by GoReleaser (-ldflags "-X github.com/iamolayemi/dustit/cmd.version=…").
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newRootCmd() *cobra.Command {
	var noColor bool
	root := &cobra.Command{
		Use:   "dustit",
		Short: "Deep-clean your Mac, free disk space and find what slows it down",
		Long: `DustIt finds caches, logs, stale project dependencies, old and duplicate files,
leftovers from deleted apps and unused Docker data, and removes them when you say so.

Nothing is deleted without asking unless you pass --yes. Start with:

  dustit scan`,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			migrateOldName()
			history.Begin(cmd.CommandPath())
			// A broken config must not lock you out of fixing it.
			if err := applyConfig(); err != nil && !strings.HasPrefix(cmd.CommandPath(), "dustit config") {
				return fmt.Errorf("%w\nFix it with: dustit config edit", err)
			}
			if noColor || os.Getenv("NO_COLOR") != "" {
				lipgloss.SetColorProfile(termenv.Ascii)
			}
			if runtime.GOOS != "darwin" {
				fmt.Fprintln(os.Stderr, "warning: DustIt is built for macOS; most paths won't exist here")
			}
			return nil
		},
	}
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colours")

	root.AddGroup(
		&cobra.Group{ID: "clean", Title: "Clean up:"},
		&cobra.Group{ID: "find", Title: "Find what uses space:"},
		&cobra.Group{ID: "speed", Title: "Speed, privacy & security:"},
	)
	for _, c := range []*cobra.Command{newConfigCmd(), newScheduleCmd(), newAlertCmd()} {
		c.GroupID = "clean"
		root.AddCommand(c)
	}
	root.AddCommand(newGenCmd(root))
	for _, c := range []*cobra.Command{newDoctorCmd(), newPrivacyCmd()} {
		c.GroupID = "speed"
		root.AddCommand(c)
	}
	for _, c := range []*cobra.Command{newScanCmd(), newCleanCmd(), newUICmd(), newListCmd(), newDockerCmd(), newHistoryCmd(), newUndoCmd()} {
		c.GroupID = "clean"
		root.AddCommand(c)
	}
	for _, c := range []*cobra.Command{newStorageCmd(), newStartupCmd(), newReportCmd(), newTopCmd(), newLargeCmd(), newOldCmd(), newProjectsCmd(),
		newDupesCmd(), newAppsCmd(), newLeftoversCmd()} {
		c.GroupID = "find"
		root.AddCommand(c)
	}
	return root
}

// errQuietExit makes dustit exit with status 1 without printing an error
// (the command already explained the outcome).
var errQuietExit = errors.New("exit 1")

// Execute runs the CLI.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		if !errors.Is(err, errQuietExit) {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(1)
	}
}
