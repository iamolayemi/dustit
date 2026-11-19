package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/targets"
	"github.com/iamolayemi/macclean/internal/ui"
)

func newUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ui",
		Short: "Pick what to clean in an interactive, full-screen list",
		Long: `Scans everything, then lets you tick categories and look inside them before cleaning.

  ↑/↓ or j/k   move            space   select / unselect
  → or l       look inside      a       select every safe category
  enter        clean selected   q       quit without changes`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if !term.IsTerminal(int(os.Stdout.Fd())) || !term.IsTerminal(int(os.Stdin.Fd())) {
				return errors.New("macclean ui needs an interactive terminal (use scan/clean in scripts)")
			}
			stop := ui.Spin("Scanning… (read-only, nothing is deleted)")
			all := targets.ScanAll(targets.All())
			stop()

			var results []targets.Result
			for _, r := range all {
				if r.HasWork() {
					results = append(results, r)
				}
			}
			if len(results) == 0 {
				fmt.Println("Nothing to clean — your Mac is already tidy.")
				return nil
			}
			m := newPicker(results)
			final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
			if err != nil {
				return err
			}
			p := final.(picker)
			if !p.confirmed {
				fmt.Println("Nothing was changed.")
				return nil
			}
			return cleanSelected(p)
		},
	}
}

type picker struct {
	results   []targets.Result
	selected  map[int]bool
	cursor    int
	detail    bool // looking inside the category under the cursor
	asking    bool // "clean these?" prompt
	confirmed bool
	height    int
}

func newPicker(rs []targets.Result) picker {
	sel := map[int]bool{}
	for i, r := range rs {
		sel[i] = r.T.Risk == targets.Safe && r.SizeKnown()
	}
	return picker{results: rs, selected: sel, height: 30}
}

func (p picker) Init() tea.Cmd { return nil }

func (p picker) total() (n int, bytes int64) {
	for i, r := range p.results {
		if p.selected[i] {
			n++
			bytes += r.Bytes
		}
	}
	return
}

func (p picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.height = msg.Height
	case tea.KeyMsg:
		key := msg.String()
		if p.asking {
			switch key {
			case "y", "Y", "enter":
				p.confirmed = true
				return p, tea.Quit
			default:
				p.asking = false
			}
			return p, nil
		}
		if p.detail {
			if key == "q" || key == "ctrl+c" {
				return p, tea.Quit
			}
			p.detail = false // any other key goes back
			return p, nil
		}
		switch key {
		case "q", "esc", "ctrl+c":
			return p, tea.Quit
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "down", "j":
			if p.cursor < len(p.results)-1 {
				p.cursor++
			}
		case " ", "x":
			p.selected[p.cursor] = !p.selected[p.cursor]
		case "a":
			for i, r := range p.results {
				if r.T.Risk == targets.Safe {
					p.selected[i] = true
				}
			}
		case "right", "l":
			p.detail = true
		case "enter":
			if n, _ := p.total(); n > 0 {
				p.asking = true
			}
		}
	}
	return p, nil
}

var (
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	titleStyle  = lipgloss.NewStyle().Bold(true).MarginBottom(1)
	helpStyle   = lipgloss.NewStyle().Faint(true)
)

func (p picker) View() string {
	var b strings.Builder
	if p.detail {
		r := p.results[p.cursor]
		b.WriteString(titleStyle.Render(fmt.Sprintf("%s — %s", r.T.Name, ui.Human(r.Bytes))) + "\n")
		b.WriteString(helpStyle.Render(r.T.Desc) + "\n\n")
		if len(r.T.CleanCmd) > 0 {
			b.WriteString("Runs: " + strings.Join(r.T.CleanCmd, " ") + "\n")
		}
		limit := max(p.height-8, 5)
		for i, it := range r.Items {
			if i == limit {
				b.WriteString(helpStyle.Render(fmt.Sprintf("  … and %d more", len(r.Items)-limit)) + "\n")
				break
			}
			note := ""
			if it.Note != "" {
				note = helpStyle.Render("  (" + it.Note + ")")
			}
			fmt.Fprintf(&b, "  %9s  %s%s\n", ui.Human(it.Size), fsx.Pretty(it.Path), note)
		}
		b.WriteString("\n" + helpStyle.Render("any key: back   q: quit"))
		return b.String()
	}

	b.WriteString(titleStyle.Render("macclean — choose what to clean") + "\n")
	for i, r := range p.results {
		cur := "  "
		if i == p.cursor {
			cur = cursorStyle.Render("› ")
		}
		box := "[ ]"
		if p.selected[i] {
			box = ui.Green.Render("[x]")
		}
		risk := ui.Green.Render("safe  ")
		if r.T.Risk == targets.Review {
			risk = ui.Yellow.Render("review")
		}
		size := ui.Human(r.Bytes)
		if !r.SizeKnown() {
			size = "?"
		}
		line := fmt.Sprintf("%s%s %10s  %s  %s", cur, box, size, risk, r.T.Name)
		if i == p.cursor {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		b.WriteString(line + "\n")
	}
	n, bytes := p.total()
	b.WriteString(fmt.Sprintf("\n%d selected · %s\n", n, ui.Bold.Render(ui.Human(bytes))))
	if p.asking {
		b.WriteString(ui.Yellow.Render(fmt.Sprintf("Clean %d categories (%s)? [Y/n]", n, ui.Human(bytes))))
	} else {
		b.WriteString(helpStyle.Render("↑/↓ move · space select · a all safe · → look inside · enter clean · q quit"))
	}
	return b.String()
}

func cleanSelected(p picker) error {
	before := fsx.FreeSpace()
	var total int64
	for i, r := range p.results {
		if !p.selected[i] {
			continue
		}
		fmt.Printf("%s (%s)\n", ui.Bold.Render(r.T.Name), ui.Human(r.Bytes))
		freed, failed := targets.Clean(r, targets.CleanOptions{Log: func(l string) { fmt.Println(ui.Dim.Render("  " + l)) }})
		total += freed
		msg := ui.Green.Render("  ✓ ") + ui.Human(freed)
		if failed > 0 {
			msg += ui.Yellow.Render(fmt.Sprintf("  (%d skipped)", failed))
		}
		fmt.Println(msg)
	}
	fmt.Printf("\nFreed about %s. Free space: %s → %s\n", ui.Bold.Render(ui.Human(total)), ui.Human(before), ui.Bold.Render(ui.Human(fsx.FreeSpace())))
	ui.Hint("Undo anything that went to the Trash with: macclean undo")
	return nil
}
