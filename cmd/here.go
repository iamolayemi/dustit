package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/dustit/internal/finder"
	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/sh"
	"github.com/iamolayemi/dustit/internal/ui"
)

func newHereCmd() *cobra.Command {
	var clean bool
	c := &cobra.Command{
		Use:   "here [path]",
		Short: "Everything the current project uses: dependencies, build output, git and Docker",
		Long: `Run inside a project (or pass its path) to see what it takes up: dependency and build
folders at any depth, the size of .git, merged branches and worktrees, and its Docker Compose
containers, images and volumes. --clean asks about each group separately.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			dir = pathArg(args, dir)
			stop := ui.Spin("Looking at " + fsx.Pretty(dir) + " …")
			p := finder.InspectProject(dir)
			stop()

			fmt.Printf("\n%s\n", ui.Bold.Render(fsx.Pretty(p.Root)))
			deps := finder.Total(p.Artifacts)
			fmt.Printf("\n  %s %s\n", ui.Bold.Render("Dependencies & build output"), ui.Dim.Render(ui.Human(deps)))
			if len(p.Artifacts) == 0 {
				fmt.Println(ui.Dim.Render("    none"))
			}
			for _, a := range p.Artifacts {
				fmt.Printf("    %9s  %s\n", ui.Human(a.Size), strings.TrimPrefix(a.Path, p.Root+"/"))
			}
			if p.GitSize > 0 {
				fmt.Printf("\n  %s %s\n", ui.Bold.Render("Git"), ui.Dim.Render(ui.Human(p.GitSize)+" in .git"))
				if p.Branches != nil {
					fmt.Println("    " + p.Branches.Note)
				}
				for _, w := range p.Worktrees {
					fmt.Printf("    %9s  worktree %s\n", ui.Human(w.Size), fsx.Pretty(w.Path))
				}
			}
			if d := p.Docker; d != nil {
				fmt.Printf("\n  %s %s\n", ui.Bold.Render("Docker"), ui.Dim.Render("Compose project "+p.Compose))
				fmt.Printf("    %d containers, %d images (%s), %d volumes\n", len(d.Containers), len(d.Images), ui.Human(d.ImageBytes), len(d.Volumes))
			}
			fmt.Println()
			if !clean {
				ui.Hint("Clean it up with: dustit here --clean")
				return nil
			}

			if len(p.Artifacts) > 0 {
				offerRemoval(p.Artifacts, false, "dependency/build folders")
			}
			if p.GitSize > 200e6 && ui.Confirm(fmt.Sprintf("Run git gc on the repository (%s in .git)?", ui.Human(p.GitSize))) {
				before := p.GitSize
				_ = sh.Shown("git", "-C", p.Root, "gc", "--quiet")
				fmt.Printf("  ✓ .git %s → %s\n", ui.Human(before), ui.Human(fsx.DiskUsage(p.Root+"/.git").Bytes))
			}
			if p.Branches != nil && ui.Confirm("Delete the merged branches? ("+p.Branches.Note+")") {
				_ = sh.Shown(p.Branches.Cmd...)
			}
			if d := p.Docker; d != nil {
				if ui.Confirm(fmt.Sprintf("Remove this project's Docker containers and the images it built (%s)? Volumes are kept.", ui.Human(d.ImageBytes))) {
					_ = sh.Shown("docker", "compose", "-p", p.Compose, "down", "--rmi", "local", "--remove-orphans")
				}
				if len(d.Volumes) > 0 {
					fmt.Println(ui.Yellow.Render("Volumes hold this project's data (databases, uploads): " + strings.Join(d.Volumes, ", ")))
					if ui.Confirm(ui.Red.Render("Permanently delete these volumes and their data?")) {
						_ = sh.Shown(append([]string{"docker", "volume", "rm"}, d.Volumes...)...)
					}
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&clean, "clean", false, "ask about removing each group")
	return c
}
