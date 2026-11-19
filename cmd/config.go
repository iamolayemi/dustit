package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/config"
	"github.com/iamolayemi/macclean/internal/finder"
	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/targets"
	"github.com/iamolayemi/macclean/internal/ui"
)

// applyConfig loads the config file and hands its settings to the packages that use them.
func applyConfig() error {
	if err := config.Load(); err != nil {
		return err
	}
	fsx.SetExcludes(config.Current.Exclude)
	finder.ExtraProjectRoots = config.Current.ProjectRoots
	return nil
}

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Show, create or edit your config (excludes, project folders, thresholds, custom categories)",
		Long: "macclean reads " + fsx.Pretty(config.Path()) + ` (or $MACCLEAN_CONFIG).

  macclean config          show the settings in effect
  macclean config edit     open it in $EDITOR (created from an example if missing)
  macclean config check    find syntax errors, typos and bad values
  macclean config init     write the commented example
  macclean config path     print where it lives`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return showConfig() },
	}
	c.AddCommand(
		&cobra.Command{Use: "path", Short: "Print the config file location", Args: cobra.NoArgs,
			Run: func(*cobra.Command, []string) { fmt.Println(config.Path()) }},
		&cobra.Command{Use: "init", Short: "Write a commented example config", Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				p, err := config.Init()
				if err != nil {
					return err
				}
				fmt.Println("Created " + fsx.Pretty(p) + ". Edit it with: macclean config edit")
				return nil
			}},
		&cobra.Command{Use: "check", Aliases: []string{"validate", "lint"}, Short: "Check the config for syntax errors, typos and bad values", Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				if checkConfig() {
					return nil
				}
				return errors.New("config has errors")
			}},
		&cobra.Command{Use: "show", Short: "Show the settings in effect", Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error { return showConfig() }},
		&cobra.Command{Use: "edit", Short: "Open the config in your editor and check it after saving", Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error { return editConfig() }},
	)
	return c
}

func editConfig() error {
	p := config.Path()
	if !fsx.Exists(p) {
		if _, err := config.Init(); err != nil {
			return err
		}
		ui.Hint("Created %s from the example.", fsx.Pretty(p))
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	for {
		var cmd *exec.Cmd
		if editor != "" {
			// $EDITOR may carry flags, e.g. "code --wait".
			cmd = exec.Command("sh", "-c", editor+` "$1"`, "sh", p)
		} else {
			ui.Hint("Opening in TextEdit (set $EDITOR to use another editor). Quit TextEdit when done.")
			cmd = exec.Command("open", "-W", "-t", p)
		}
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("editor: %w", err)
		}
		if checkConfig() {
			if err := applyConfig(); err != nil {
				return err
			}
			fmt.Println()
			return showConfig()
		}
		if !ui.Confirm("Open it again to fix?") {
			return errors.New("config has errors")
		}
	}
}

// checkConfig prints problems and reports whether the config is usable (no errors).
func checkConfig() bool {
	probs, err := config.Check(targets.BuiltinIDs())
	if err != nil {
		ui.Err("%v", err)
		return false
	}
	nErr := 0
	for _, p := range probs {
		if p.Error {
			nErr++
			fmt.Println(ui.Red.Render("✗ ") + p.String())
		} else {
			fmt.Println(ui.Yellow.Render("! ") + p.String())
		}
	}
	switch {
	case nErr > 0:
		fmt.Printf("\n%s has %d error(s).\n", fsx.Pretty(config.Path()), nErr)
		return false
	case len(probs) > 0:
		fmt.Println(ui.Green.Render("✓") + " Config is valid (with warnings).")
	default:
		fmt.Println(ui.Green.Render("✓") + " Config is valid.")
	}
	return true
}

func showConfig() error {
	c := config.Current
	p := config.Path()
	state := ui.Dim.Render("(not created yet — defaults in use; run: macclean config edit)")
	if fsx.Exists(p) {
		state = ""
	}
	fmt.Printf("%s %s\n\n", ui.Bold.Render(fsx.Pretty(p)), state)

	list := func(title string, items []string, empty string) {
		fmt.Println(ui.Bold.Render(title))
		if len(items) == 0 {
			fmt.Println("  " + ui.Dim.Render(empty))
		}
		for _, it := range items {
			fmt.Println("  " + it)
		}
		fmt.Println()
	}
	list("Never remove (exclude):", c.Exclude, "nothing excluded")
	var roots []string
	for _, r := range finder.ProjectRoots() {
		roots = append(roots, fsx.Pretty(r))
	}
	list("Project folders searched:", roots, "none found")
	fmt.Println(ui.Bold.Render("Thresholds:"))
	fmt.Printf("  stale_days     %d\n  old_file_days  %d\n  old_file_min   %s\n  duplicate_min  %s\n  download_days  %d\n\n",
		c.StaleDays, c.OldFileDays, c.OldFileMin, c.DuplicateMin, c.DownloadDays)
	list("Disabled categories:", c.Disabled, "none")
	var cats []string
	for _, cat := range c.Categories {
		risk := cat.Risk
		if risk == "" {
			risk = "review"
		}
		cats = append(cats, fmt.Sprintf("%-20s %-7s %s", cat.ID, risk, strings.Join(cat.Paths, ", ")))
	}
	list("Custom categories:", cats, "none")
	return nil
}
