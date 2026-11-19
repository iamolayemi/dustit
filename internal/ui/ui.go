// Package ui holds terminal styling, formatting and prompts.
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/dustin/go-humanize"
	"golang.org/x/term"
)

var (
	Bold   = lipgloss.NewStyle().Bold(true)
	Dim    = lipgloss.NewStyle().Faint(true)
	Red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	Green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	Yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	Cyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

// Human formats bytes the way Finder does (base 1000, one decimal).
func Human(n int64) string {
	if n < 0 {
		n = 0
	}
	if n < 1000 {
		return fmt.Sprintf("%d B", n)
	}
	f := float64(n)
	for _, u := range []string{"KB", "MB", "GB", "TB"} {
		f /= 1000
		if f < 1000 {
			return fmt.Sprintf("%.1f %s", f, u)
		}
	}
	return fmt.Sprintf("%.1f PB", f/1000)
}

// ParseSize accepts sizes like "500MB", "1.5G", "200".
func ParseSize(s string) (int64, error) {
	n, err := humanize.ParseBytes(s)
	return int64(n), err
}

// Ago is "3 months ago".
func Ago(t time.Time) string { return humanize.Time(t) }

// Since is "3 months" (no "ago").
func Since(t time.Time) string { return humanize.RelTime(t, time.Now(), "", "") }

// Bar draws a proportional bar.
func Bar(n, max int64, width int) string {
	if max <= 0 {
		return strings.Repeat(" ", width)
	}
	w := int(float64(n) / float64(max) * float64(width))
	if w < 1 && n > 0 {
		w = 1
	}
	return Cyan.Render(strings.Repeat("█", w)) + strings.Repeat(" ", width-w)
}

// Hint prints a dim line.
func Hint(format string, a ...any) {
	for _, line := range strings.Split(fmt.Sprintf(format, a...), "\n") {
		fmt.Println(Dim.Render(line))
	}
}

// Warn prints a yellow line.
func Warn(format string, a ...any) { fmt.Println(Yellow.Render(fmt.Sprintf(format, a...))) }

// Err prints a red line.
func Err(format string, a ...any) { fmt.Println(Red.Render(fmt.Sprintf(format, a...))) }

// FDATip explains how to let the terminal read protected folders.
const FDATip = "Some folders couldn't be read. To include them, give your terminal app Full Disk Access:\n  System Settings → Privacy & Security → Full Disk Access."

var stdin = bufio.NewReader(os.Stdin)

// Ask prints q and returns the trimmed, lower-cased answer ("" on EOF).
func Ask(q string) string {
	fmt.Print(q)
	line, _ := stdin.ReadString('\n')
	return strings.ToLower(strings.TrimSpace(line))
}

// Confirm asks a yes/no question; anything but y/yes is no.
func Confirm(q string) bool {
	a := Ask(q + " [y/N] ")
	return a == "y" || a == "yes"
}

// Spin shows a spinner on stderr while work runs; call the returned func to stop it.
func Spin(msg string) func() {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		fmt.Fprintln(os.Stderr, msg)
		return func() {}
	}
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
	s.Suffix = " " + msg
	s.Start()
	return s.Stop
}

// Table renders rows with a header; style picks the style per cell (row -1 = header).
func Table(headers []string, rows [][]string, style func(row, col int) lipgloss.Style) string {
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(Dim).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			base := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return base.Bold(true)
			}
			if style != nil {
				return style(row, col).Padding(0, 1)
			}
			return base
		})
	return t.Render()
}
