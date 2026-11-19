package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"

	"github.com/iamolayemi/dustit/internal/finder"
	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/ui"
)

// sizeFlag is a pflag.Value that accepts "500MB", "1.5G", …
type sizeFlag struct{ n int64 }

func newSize(def string) *sizeFlag {
	n, _ := ui.ParseSize(def)
	return &sizeFlag{n}
}
func (s *sizeFlag) String() string { return ui.Human(s.n) }
func (s *sizeFlag) Type() string   { return "size" }
func (s *sizeFlag) Set(v string) error {
	n, err := ui.ParseSize(v)
	if err != nil {
		return fmt.Errorf("bad size %q (try 500MB or 1.5GB)", v)
	}
	s.n = n
	return nil
}

var _ pflag.Value = (*sizeFlag)(nil)

// pathArg returns the first argument as an absolute path, or def.
func pathArg(args []string, def string) string {
	if len(args) > 0 {
		return fsx.Expand(args[0])
	}
	return def
}

func prettyList(paths []string) string {
	var out []string
	for _, p := range paths {
		out = append(out, fsx.Pretty(p))
	}
	return strings.Join(out, ", ")
}

// printItems lists items as "size  meta  path".
func printItems(items []finder.Item, meta func(finder.Item) string) {
	fmt.Println()
	for _, it := range items {
		m := ""
		if meta != nil {
			m = meta(it)
		}
		fmt.Printf("  %9s  %s %s\n", ui.Human(it.Size), ui.Dim.Render(fmt.Sprintf("%-16s", m)), fsx.Pretty(it.Path))
	}
	fmt.Println()
}

func trashHint(toTrash bool, freed int64) {
	if toTrash && freed > 0 {
		ui.Hint("Space is freed when you empty the Trash (dustit clean --only trash).")
	}
}

// removeAll removes every item after one confirmation was given.
func removeAll(items []finder.Item, toTrash bool) {
	var freed int64
	failed := 0
	for _, it := range items {
		if err := fsx.RemoveSized(it.Path, toTrash, it.Size); err != nil {
			failed++
			ui.Err("  %v", err)
			continue
		}
		freed += it.Size
	}
	msg := "Removed " + ui.Bold.Render(ui.Human(freed))
	if failed > 0 {
		msg += ui.Yellow.Render(fmt.Sprintf(" (%d failed)", failed))
	}
	fmt.Println(msg + ".")
	trashHint(toTrash, freed)
}

// reviewEach asks about every item individually.
func reviewEach(items []finder.Item, toTrash bool) {
	verb := "Move to Trash"
	if !toTrash {
		verb = ui.Red.Render("Permanently delete")
	}
	var freed int64
	for _, it := range items {
		a := ui.Ask(fmt.Sprintf("%s %s (%s)? [y/N/q] ", verb, fsx.Pretty(it.Path), ui.Human(it.Size)))
		if a == "q" {
			break
		}
		if a != "y" && a != "yes" {
			continue
		}
		if err := fsx.RemoveSized(it.Path, toTrash, it.Size); err != nil {
			ui.Err("  %v", err)
			continue
		}
		freed += it.Size
	}
	fmt.Printf("\nRemoved %s.\n", ui.Bold.Render(ui.Human(freed)))
	trashHint(toTrash, freed)
}

// offerRemoval asks once for all items, with "e" to choose each.
func offerRemoval(items []finder.Item, toTrash bool, what string) {
	verb := "Move all %d %s to Trash (%s)? [y/N/e = choose each] "
	if !toTrash {
		verb = "Delete all %d %s (%s)? [y/N/e = choose each] "
	}
	switch ui.Ask(fmt.Sprintf(verb, len(items), what, ui.Human(finder.Total(items)))) {
	case "y", "yes":
		removeAll(items, toTrash)
	case "e":
		reviewEach(items, toTrash)
	}
}
