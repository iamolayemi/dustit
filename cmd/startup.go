package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/macclean/internal/finder"
	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/ui"
)

const loginItemsPane = "x-apple.systempreferences:com.apple.LoginItems-Settings.extension"

func newStartupCmd() *cobra.Command {
	var disable, enable string
	var removeOrphans, open, asJSON bool
	c := &cobra.Command{
		Use:   "startup",
		Short: "Background items and login agents, with the ones left by removed apps first",
		Long: `Lists every launch agent and daemon — what starts in the background, which app it
belongs to, whether it's running — with items whose program no longer exists first.

  macclean startup                          list everything
  macclean startup --disable <label>        stop one of your agents and keep it from starting
  macclean startup --enable <label>         undo --disable
  macclean startup --remove-orphans         unload and trash your agents whose program is gone
  macclean startup --open                   open Login Items & Extensions in System Settings

Items for all users or the system need an admin password; macclean prints the exact sudo
command instead of running it.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if open {
				return exec.Command("open", loginItemsPane).Run()
			}
			stop := ui.Spin("Reading background items…")
			items := finder.StartupItems()
			stop()
			switch {
			case disable != "":
				return setStartup(items, disable, false)
			case enable != "":
				return setStartup(items, enable, true)
			case removeOrphans:
				return removeOrphanAgents(items)
			case asJSON:
				return writeJSON(items)
			}

			orphans := 0
			scope := ""
			for _, it := range items {
				if it.Orphaned {
					orphans++
				}
				if !it.Orphaned && it.Scope != scope {
					scope = it.Scope
					fmt.Println("\n" + ui.Bold.Render(scopeTitle(scope)))
				} else if it.Orphaned && orphans == 1 {
					fmt.Println("\n" + ui.Bold.Render(ui.Yellow.Render("Left behind (their program no longer exists)")))
				}
				state := ui.Dim.Render("not running")
				switch {
				case it.Disabled:
					state = ui.Dim.Render("disabled")
				case it.Loaded:
					state = ui.Green.Render("running")
				}
				owner := it.Owner
				if owner == "" {
					owner = "—"
				}
				fmt.Printf("  %-12s %-30s %s\n", state, truncate(owner, 30), it.Label)
				if it.Orphaned {
					fmt.Println("               " + ui.Dim.Render(it.Reason))
				}
			}
			fmt.Println()
			if orphans > 0 {
				ui.Hint("Remove the ones left behind: macclean startup --remove-orphans")
			}
			ui.Hint("Stop one from starting: macclean startup --disable <label>")
			ui.Hint("Apps' own login items: macclean startup --open (System Settings → Login Items & Extensions)")
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&disable, "disable", "", "stop an agent and keep it from starting")
	f.StringVar(&enable, "enable", "", "allow a disabled agent to start again")
	f.BoolVar(&removeOrphans, "remove-orphans", false, "unload and move to Trash your agents whose program no longer exists")
	f.BoolVar(&open, "open", false, "open Login Items & Extensions in System Settings")
	f.BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return c
}

func scopeTitle(s string) string {
	switch s {
	case finder.ScopeUser:
		return "Your launch agents (~/Library/LaunchAgents)"
	case finder.ScopeAllAgents:
		return "Launch agents for all users (/Library/LaunchAgents)"
	}
	return "System daemons (/Library/LaunchDaemons)"
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func findStartup(items []finder.StartupItem, label string) (finder.StartupItem, error) {
	for _, it := range items {
		if it.Label == label {
			return it, nil
		}
	}
	return finder.StartupItem{}, fmt.Errorf("no background item with label %q (see: macclean startup)", label)
}

func setStartup(items []finder.StartupItem, label string, on bool) error {
	it, err := findStartup(items, label)
	if err != nil {
		return err
	}
	if it.Scope != finder.ScopeUser {
		domain := "system"
		verb := "disable"
		if on {
			verb = "enable"
		}
		fmt.Println("This item belongs to all users, so it needs an admin password. Run:")
		fmt.Printf("  sudo launchctl %s %s/%s\n", verb, domain, it.Label)
		if !on {
			fmt.Printf("  sudo launchctl bootout %s %q\n", domain, it.Plist)
		}
		return nil
	}
	target := finder.UserDomain() + "/" + it.Label
	if on {
		if err := exec.Command("launchctl", "enable", target).Run(); err != nil {
			return err
		}
		_ = exec.Command("launchctl", "bootstrap", finder.UserDomain(), it.Plist).Run()
		fmt.Println(ui.Green.Render("✓") + " " + it.Label + " will start again.")
		return nil
	}
	_ = exec.Command("launchctl", "bootout", target).Run() // fine if it wasn't running
	if err := exec.Command("launchctl", "disable", target).Run(); err != nil {
		return err
	}
	fmt.Println(ui.Green.Render("✓") + " " + it.Label + " is stopped and won't start again. Undo: macclean startup --enable " + it.Label)
	return nil
}

func removeOrphanAgents(items []finder.StartupItem) error {
	var mine, others []finder.StartupItem
	for _, it := range items {
		if !it.Orphaned {
			continue
		}
		if it.Scope == finder.ScopeUser {
			mine = append(mine, it)
		} else {
			others = append(others, it)
		}
	}
	if len(mine)+len(others) == 0 {
		fmt.Println("Nothing left behind — every background item's program still exists.")
		return nil
	}
	for _, it := range mine {
		fmt.Printf("  %s  %s\n", it.Label, ui.Dim.Render(it.Reason))
	}
	if len(mine) > 0 && ui.Confirm(fmt.Sprintf("Unload these %d agents and move them to the Trash?", len(mine))) {
		for _, it := range mine {
			_ = exec.Command("launchctl", "bootout", finder.UserDomain()+"/"+it.Label).Run()
			if err := fsx.Remove(it.Plist, true); err != nil {
				ui.Err("  %v", err)
				continue
			}
			fmt.Println(ui.Green.Render("  ✓ ") + it.Label)
		}
	}
	if len(others) > 0 {
		fmt.Println("\nThese belong to all users and need an admin password:")
		var cmds []string
		for _, it := range others {
			domain := "system"
			cmds = append(cmds, fmt.Sprintf("sudo launchctl bootout %s %q; sudo rm %q", domain, it.Plist, it.Plist))
		}
		fmt.Println("  " + strings.Join(cmds, "\n  "))
	}
	if len(mine) == 0 && len(others) == 0 {
		return errors.New("nothing to remove")
	}
	return nil
}
