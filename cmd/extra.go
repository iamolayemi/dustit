package cmd

import "github.com/spf13/cobra"

// extraFindCommands are the "find what uses space" commands added after the first set.
func extraFindCommands() []*cobra.Command {
	return []*cobra.Command{newHereCmd(), newExploreCmd(), newGrowthCmd(), newSnapshotCmd()}
}
