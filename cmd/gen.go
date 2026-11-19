package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newGenCmd writes man pages and shell completions; used by the release build.
func newGenCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "gen <dir>",
		Short:  "Generate man pages and shell completions into <dir>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			man := filepath.Join(args[0], "man")
			comp := filepath.Join(args[0], "completions")
			for _, d := range []string{man, comp} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					return err
				}
			}
			root.DisableAutoGenTag = true
			if err := doc.GenManTree(root, &doc.GenManHeader{Title: "DUSTIT", Section: "1", Source: "dustit " + version}, man); err != nil {
				return err
			}
			gens := map[string]func(string) error{
				"dustit.bash": root.GenBashCompletionFile,
				"_dustit":     root.GenZshCompletionFile,
				"dustit.fish": func(p string) error { return root.GenFishCompletionFile(p, true) },
			}
			for name, gen := range gens {
				if err := gen(filepath.Join(comp, name)); err != nil {
					return err
				}
			}
			fmt.Println("wrote", man, "and", comp)
			return nil
		},
	}
}
