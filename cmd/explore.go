package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/iamolayemi/dustit/internal/finder"
	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/ui"
)

func newExploreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "explore [path]",
		Short: "Browse folders by size in a full-screen view and remove what you don't need",
		Long: `A full-screen folder browser sorted by size (like ncdu).

  ↑/↓ j/k   move             → l enter   open folder
  ← h ⌫     go up            d           move to Trash (asks first; undo with dustit undo)
  o         show in Finder   r           measure again
  q         quit`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdout.Fd())) || !term.IsTerminal(int(os.Stdin.Fd())) {
				return errors.New("dustit explore needs an interactive terminal (try dustit top)")
			}
			start := pathArg(args, fsx.Home)
			if !fsx.IsDir(start) {
				return fmt.Errorf("%s isn't a folder", start)
			}
			_, err := tea.NewProgram(newExplorer(start), tea.WithAltScreen()).Run()
			return err
		},
	}
}

type explorer struct {
	dir     string
	items   []finder.Item
	cursor  map[string]int
	cache   map[string][]finder.Item
	loading bool
	confirm bool
	status  string
	height  int
}

type measured struct {
	dir   string
	items []finder.Item
	err   error
}

func newExplorer(dir string) explorer {
	return explorer{dir: dir, cursor: map[string]int{}, cache: map[string][]finder.Item{}, height: 30, loading: true}
}

func measure(dir string) tea.Cmd {
	return func() tea.Msg {
		items, _, err := finder.Children(dir)
		return measured{dir, items, err}
	}
}

func (m explorer) Init() tea.Cmd { return measure(m.dir) }

func (m explorer) open(dir string) (explorer, tea.Cmd) {
	m.dir, m.confirm, m.status = dir, false, ""
	if items, ok := m.cache[dir]; ok {
		m.items, m.loading = items, false
		return m, nil
	}
	m.items, m.loading = nil, true
	return m, measure(dir)
}

func (m explorer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case measured:
		m.cache[msg.dir] = msg.items
		if msg.dir == m.dir {
			m.items, m.loading = msg.items, false
			if msg.err != nil {
				m.status = msg.err.Error()
			}
		}
	case tea.KeyMsg:
		cur := m.cursor[m.dir]
		key := msg.String()
		if m.confirm {
			m.confirm = false
			if key == "y" || key == "Y" {
				it := m.items[cur]
				if err := fsx.RemoveSized(it.Path, true, it.Size); err != nil {
					m.status = err.Error()
				} else {
					m.status = "Moved " + filepath.Base(it.Path) + " to the Trash (" + ui.Human(it.Size) + "). Undo: dustit undo"
					m.items = append(append([]finder.Item(nil), m.items[:cur]...), m.items[cur+1:]...)
					m.cache[m.dir] = m.items
					delete(m.cache, filepath.Dir(m.dir)) // parent totals changed
					if cur >= len(m.items) && cur > 0 {
						m.cursor[m.dir] = cur - 1
					}
				}
			} else {
				m.status = ""
			}
			return m, nil
		}
		switch key {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if cur > 0 {
				m.cursor[m.dir] = cur - 1
			}
		case "down", "j":
			if cur < len(m.items)-1 {
				m.cursor[m.dir] = cur + 1
			}
		case "right", "l", "enter":
			if cur < len(m.items) && fsx.IsDir(m.items[cur].Path) {
				return m.open(m.items[cur].Path)
			}
		case "left", "h", "backspace":
			if m.dir != "/" {
				return m.open(filepath.Dir(m.dir))
			}
		case "r":
			delete(m.cache, m.dir)
			return m.open(m.dir)
		case "o":
			if cur < len(m.items) {
				_ = exec.Command("open", "-R", m.items[cur].Path).Start()
			}
		case "d":
			if cur < len(m.items) {
				if err := fsx.CheckDeletable(m.items[cur].Path); err != nil {
					m.status = err.Error()
				} else {
					m.confirm = true
				}
			}
		}
	}
	return m, nil
}

var (
	exHeader = lipgloss.NewStyle().Bold(true)
	exSel    = lipgloss.NewStyle().Reverse(true)
	exDim    = lipgloss.NewStyle().Faint(true)
)

func (m explorer) View() string {
	var b strings.Builder
	total := finder.Total(m.items)
	b.WriteString(exHeader.Render(fsx.Pretty(m.dir)) + "  " + exDim.Render(ui.Human(total)) + "\n\n")
	if m.loading {
		b.WriteString(exDim.Render("  Measuring… (large folders take a moment)") + "\n")
	}
	rows := max(m.height-6, 5)
	cur := m.cursor[m.dir]
	start := 0
	if cur >= rows {
		start = cur - rows + 1
	}
	var maxSize int64
	if len(m.items) > 0 {
		maxSize = m.items[0].Size
	}
	for i := start; i < len(m.items) && i < start+rows; i++ {
		it := m.items[i]
		name := filepath.Base(it.Path)
		if fsx.IsDir(it.Path) {
			name += "/"
		}
		line := fmt.Sprintf(" %9s  %s  %s", ui.Human(it.Size), ui.Bar(it.Size, maxSize, 18), name)
		if i == cur {
			line = exSel.Render(line)
		}
		b.WriteString(line + "\n")
	}
	if !m.loading && len(m.items) == 0 {
		b.WriteString(exDim.Render("  (empty)") + "\n")
	}
	b.WriteString("\n")
	switch {
	case m.confirm:
		it := m.items[cur]
		b.WriteString(ui.Yellow.Render(fmt.Sprintf("Move %s (%s) to the Trash? [y/N]", filepath.Base(it.Path), ui.Human(it.Size))))
	case m.status != "":
		b.WriteString(m.status)
	default:
		b.WriteString(exDim.Render("↑/↓ move · → open · ← up · d trash · o show in Finder · r refresh · q quit"))
	}
	return b.String()
}
