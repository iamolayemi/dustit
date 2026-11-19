package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/iamolayemi/dustit/internal/config"
	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/ui"
)

// oldName is what DustIt was called before its first release.
const oldName = "macclean"

// migrateOldName moves settings and history saved under the old name to the new folders,
// and unloads background jobs installed under the old name. It's a no-op once done.
func migrateOldName() {
	var moves [][2]string
	if os.Getenv("DUSTIT_CONFIG") == "" {
		newDir := filepath.Dir(config.Path())
		moves = append(moves, [2]string{filepath.Join(filepath.Dir(newDir), oldName), newDir})
	}
	support := fsx.H("Library/Application Support")
	moves = append(moves, [2]string{filepath.Join(support, oldName), filepath.Join(support, "dustit")})
	for _, m := range moves {
		if fsx.Exists(m[0]) && !fsx.Exists(m[1]) {
			if err := os.Rename(m[0], m[1]); err == nil {
				fmt.Fprintln(os.Stderr, ui.Dim.Render("Moved "+fsx.Pretty(m[0])+" to "+fsx.Pretty(m[1])+" (DustIt was called macclean before)."))
			}
		}
	}
	for _, suffix := range []string{"", ".alert"} {
		old := fsx.H("Library/LaunchAgents/com.github.iamolayemi." + oldName + suffix + ".plist")
		if !fsx.Exists(old) {
			continue
		}
		_ = exec.Command("launchctl", "bootout", "gui/"+fmt.Sprint(os.Getuid()), old).Run()
		_ = os.Remove(old)
		what := "scheduled clean-up"
		again := "dustit schedule on"
		if suffix == ".alert" {
			what, again = "low-disk alert", "dustit schedule alert on"
		}
		fmt.Fprintf(os.Stderr, "Your %s was set up under the old name and has been turned off. Turn it on again with: %s\n", what, again)
	}
}
