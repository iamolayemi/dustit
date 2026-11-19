package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"gopkg.in/yaml.v3"
)

// Problem is something wrong (Error) or suspicious (Warning) in the config file.
type Problem struct {
	Error bool
	Line  int // 0 when unknown
	Msg   string
}

func (p Problem) String() string {
	loc := ""
	if p.Line > 0 {
		loc = fmt.Sprintf("line %d: ", p.Line)
	}
	return loc + p.Msg
}

var yamlLine = regexp.MustCompile(`line (\d+)`)

func lineOf(err error) int {
	var n int
	if m := yamlLine.FindStringSubmatch(err.Error()); m != nil {
		_, _ = fmt.Sscan(m[1], &n)
	}
	return n
}

// Check validates the config file strictly: syntax, unknown keys (typos), types and values.
// knownIDs are the built-in category ids, used to spot typos in `disabled` and clashes.
func Check(knownIDs []string) ([]Problem, error) {
	b, err := os.ReadFile(Path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s doesn't exist yet — create it with: macclean config edit", Path())
	}
	if err != nil {
		return nil, err
	}
	var probs []Problem
	add := func(isErr bool, line int, format string, a ...any) {
		probs = append(probs, Problem{isErr, line, fmt.Sprintf(format, a...)})
	}

	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true) // unknown keys are errors, so typos don't silently do nothing
	if err := dec.Decode(&c); err != nil && !errors.Is(err, errEOF(err)) {
		var te *yaml.TypeError
		if !errors.As(err, &te) {
			add(true, lineOf(err), "%s", cleanYAML(err.Error()))
			return probs, nil // a syntax error: nothing else can be checked
		}
		for _, e := range te.Errors {
			add(true, lineOf(errors.New(e)), "%s", explain(e))
		}
		// Type problems and unknown keys still leave the rest readable: keep checking.
		c = Config{}
		_ = yaml.Unmarshal(b, &c)
	}

	// Line numbers for the values we check further.
	var root yaml.Node
	_ = yaml.Unmarshal(b, &root)
	line := func(key string) int { return keyLine(&root, key) }

	for _, f := range []struct{ key, val string }{{"old_file_min", c.OldFileMin}, {"duplicate_min", c.DuplicateMin}} {
		if f.val != "" {
			if _, err := humanize.ParseBytes(f.val); err != nil {
				add(true, line(f.key), "%s: %q isn't a size (try 100MB or 1.5GB)", f.key, f.val)
			}
		}
	}
	for _, f := range []struct {
		key string
		v   int
	}{{"stale_days", c.StaleDays}, {"old_file_days", c.OldFileDays}, {"download_days", c.DownloadDays}} {
		if f.v < 0 {
			add(true, line(f.key), "%s must be 0 or more", f.key)
		}
	}
	for _, e := range c.Exclude {
		if strings.ContainsAny(e, "*?[") {
			if _, err := filepath.Match(expand(e), "x"); err != nil {
				add(true, line("exclude"), "exclude: bad pattern %q", e)
			}
			continue
		}
		if _, err := os.Lstat(expand(e)); err != nil {
			add(false, line("exclude"), "exclude: %s doesn't exist (still honoured if it appears later)", e)
		}
	}
	for _, r := range c.ProjectRoots {
		if fi, err := os.Stat(expand(r)); err != nil || !fi.IsDir() {
			add(false, line("project_roots"), "project_roots: %s isn't a folder", r)
		}
	}
	known := map[string]bool{}
	for _, id := range knownIDs {
		known[id] = true
	}
	custom := map[string]bool{}
	for _, cat := range c.Categories {
		custom[cat.ID] = true
	}
	for _, d := range c.Disabled {
		if !known[d] && !custom[d] {
			add(false, line("disabled"), "disabled: %q isn't a category (see: macclean list)", d)
		}
	}
	seen := map[string]bool{}
	for i, cat := range c.Categories {
		n := i + 1
		switch {
		case cat.ID == "":
			add(true, line("categories"), "categories #%d: missing id", n)
		case known[cat.ID]:
			add(true, line("categories"), "categories: id %q clashes with a built-in category", cat.ID)
		case seen[cat.ID]:
			add(true, line("categories"), "categories: id %q is used twice", cat.ID)
		}
		seen[cat.ID] = true
		if len(cat.Paths) == 0 {
			add(true, line("categories"), "categories %q: needs at least one path", cat.ID)
		}
		if cat.Risk != "" && cat.Risk != "safe" && cat.Risk != "review" {
			add(true, line("categories"), "categories %q: risk must be safe or review, not %q", cat.ID, cat.Risk)
		}
		for _, p := range cat.Paths {
			full := expand(p)
			if full == filepath.Clean(home()) || !strings.HasPrefix(full, home()+"/") {
				add(true, line("categories"), "categories %q: %s must be a folder inside your home folder", cat.ID, p)
			} else if _, err := os.Stat(full); err != nil {
				add(false, line("categories"), "categories %q: %s doesn't exist", cat.ID, p)
			}
		}
	}
	sort.SliceStable(probs, func(i, j int) bool { return probs[i].Line < probs[j].Line })
	return probs, nil
}

// errEOF treats an empty file as valid.
func errEOF(err error) error {
	if err != nil && err.Error() == "EOF" {
		return err
	}
	return errors.New("not eof")
}

var (
	unknownField = regexp.MustCompile(`field (\S+) not found`)
	settingKeys  = []string{"exclude", "project_roots", "stale_days", "old_file_days", "old_file_min", "duplicate_min", "download_days", "disabled", "categories"}
	categoryKeys = []string{"id", "name", "description", "paths", "risk", "trash", "min_age_days"}
)

// explain turns yaml's error into a friendlier message, suggesting the key you probably meant.
func explain(e string) string {
	if m := unknownField.FindStringSubmatch(e); m != nil {
		keys := settingKeys
		if strings.Contains(e, "Category") {
			keys = categoryKeys
		}
		msg := fmt.Sprintf("unknown setting %q", m[1])
		if s := closest(m[1], keys); s != "" {
			msg += fmt.Sprintf(" — did you mean %q?", s)
		}
		return msg
	}
	return cleanYAML(e)
}

// closest returns the key within edit distance 3 of s, if any.
func closest(s string, keys []string) string {
	best, bestD := "", 4
	for _, k := range keys {
		if d := editDistance(s, k); d < bestD {
			best, bestD = k, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func cleanYAML(s string) string {
	s = strings.TrimPrefix(s, "yaml: ")
	s = yamlLine.ReplaceAllString(s, "")
	s = strings.TrimLeft(s, ": ")
	s = strings.ReplaceAll(s, "in type config.Config", "")
	s = strings.ReplaceAll(s, "into config.Category", "")
	return strings.TrimSpace(s)
}

func keyLine(n *yaml.Node, key string) int {
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return 0
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i].Line
		}
	}
	return 0
}

func home() string { h, _ := os.UserHomeDir(); return filepath.Clean(h) }

func expand(p string) string {
	p = strings.TrimSpace(p)
	if p == "~" {
		return home()
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home(), p[2:])
	}
	return filepath.Clean(p)
}
