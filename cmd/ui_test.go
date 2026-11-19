package cmd

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iamolayemi/macclean/internal/finder"
	"github.com/iamolayemi/macclean/internal/targets"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(m tea.Model, keys ...string) picker {
	for _, k := range keys {
		m, _ = m.Update(key(k))
	}
	return m.(picker)
}

func TestPicker(t *testing.T) {
	safe := &targets.Target{ID: "npm-cache", Name: "npm cache", Risk: targets.Safe}
	review := &targets.Target{ID: "old-files", Name: "Old files", Risk: targets.Review}
	rs := []targets.Result{
		{T: safe, Bytes: 1000, Items: []finder.Item{{Path: "/a", Size: 1000}}},
		{T: review, Bytes: 5000, Items: []finder.Item{{Path: "/b", Size: 5000}}},
	}
	p := newPicker(rs)
	if !p.selected[0] || p.selected[1] {
		t.Fatal("safe categories start selected, review ones don't")
	}
	p = press(p, "down", "space")
	if n, b := p.total(); n != 2 || b != 6000 {
		t.Fatalf("selecting the review row: got %d / %d", n, b)
	}
	p = press(p, "l")
	if !p.detail || !strings.Contains(p.View(), "/b") {
		t.Fatal("→ should show the items of the row")
	}
	p = press(p, "x") // any key goes back
	p = press(p, "enter")
	if !p.asking || !strings.Contains(p.View(), "Clean 2 categories") {
		t.Fatal("enter should ask for confirmation")
	}
	p = press(p, "n")
	if p.asking || p.confirmed {
		t.Fatal("n should cancel the prompt")
	}
	p = press(p, "enter", "y")
	if !p.confirmed {
		t.Fatal("y should confirm")
	}
}
