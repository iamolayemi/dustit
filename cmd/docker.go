package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/docker"
	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/sh"
	"github.com/iamolayemi/macclean/internal/ui"
)

func newDockerCmd() *cobra.Command {
	var clean, volumes, start, yes bool
	c := &cobra.Command{
		Use:   "docker",
		Short: "Show what's inside Docker Desktop's disk image; --clean prunes it",
		Long: `Shows what is using Docker Desktop's disk image (docker system df).

With --clean it removes stopped containers, unused images, build cache and unused
networks, asking before each step, then waits for Docker Desktop to give the freed
space back to macOS so the disk image file actually shrinks.

Volumes hold data (databases etc.), so they are only offered with --volumes, and
always ask before deleting.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !docker.Installed() {
				return errors.New("docker is not installed")
			}
			used, limit := docker.RawSizes()
			if limit > 0 {
				fmt.Printf("Docker Desktop disk image: %s on disk (can grow to %s)\n", ui.Bold.Render(ui.Human(used)), ui.Human(limit))
			}
			if !docker.Running() {
				if !start && !ui.Confirm("Docker isn't running. Start Docker Desktop to see what can be removed?") {
					return nil
				}
				stop := ui.Spin("Starting Docker Desktop…")
				ok := docker.Start(3 * time.Minute)
				stop()
				if !ok {
					return errors.New("docker didn't start within 3 minutes — open Docker Desktop and try again")
				}
			}

			fmt.Println()
			_ = sh.Shown("docker", "system", "df")
			fmt.Println()
			if !clean {
				ui.Hint("The RECLAIMABLE column is what `macclean docker --clean` can free.")
				ui.Hint("Volumes hold database data; they are only offered with --volumes.")
				return nil
			}

			for _, s := range docker.PruneSteps {
				if !yes && !ui.Confirm(s.Question) {
					continue
				}
				fmt.Println(ui.Dim.Render("  $ " + strings.Join(s.Args, " ")))
				if err := sh.Shown(s.Args...); err != nil {
					ui.Err("  failed: %v", err)
				}
			}
			if volumes {
				if names := docker.UnusedVolumes(); len(names) > 0 {
					ui.Warn("\n%d volumes are not used by any container:", len(names))
					for _, n := range names {
						fmt.Println("  " + n)
					}
					if ui.Confirm(ui.Red.Render("Permanently delete these volumes and all data in them?")) {
						if err := sh.Shown("docker", "volume", "prune", "-a", "-f"); err != nil {
							ui.Err("  failed: %v", err)
						}
					}
				}
			}

			stop := ui.Spin("Waiting for Docker Desktop to return the space to macOS…")
			after := docker.WaitForShrink(used, nil)
			stop()
			fmt.Printf("\nDocker disk image: %s → %s (freed %s). Free space: %s\n",
				ui.Human(used), ui.Bold.Render(ui.Human(after)), ui.Bold.Render(ui.Human(used-after)), ui.Human(fsx.FreeSpace()))
			if used-after < 1e9 {
				ui.Hint("If it hasn't shrunk yet, quit and reopen Docker Desktop — it compacts the image on restart.")
			}
			return nil
		},
	}
	f := c.Flags()
	f.BoolVar(&clean, "clean", false, "remove unused images, stopped containers, build cache and networks (asks before each)")
	f.BoolVar(&volumes, "volumes", false, "with --clean: also offer to remove volumes no container uses (deletes their data!)")
	f.BoolVar(&start, "start", false, "start Docker Desktop without asking if it isn't running")
	f.BoolVarP(&yes, "yes", "y", false, "don't ask before each step (volumes still ask)")
	return c
}
