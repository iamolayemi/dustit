package finder

import (
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/sh"
)

var uuidName = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}(\.jsonl)?$`)

// olderThan keeps paths (from globs relative to ~) whose newest change is older than `days`.
// Folders called "memory" are never included.
func olderThan(days int, note string, keep func(name string) bool, patterns ...string) []Item {
	cutoff := time.Now().AddDate(0, 0, -days)
	var out []Item
	for _, pat := range patterns {
		ms, _ := filepath.Glob(fsx.H(pat))
		for _, m := range ms {
			name := filepath.Base(m)
			if name == "memory" || strings.HasPrefix(name, ".") || (keep != nil && !keep(name)) {
				continue
			}
			if newest(m).Before(cutoff) {
				out = append(out, Item{Path: m, Note: note})
			}
		}
	}
	return out
}

// newest is the most recent modification inside p (p itself for files).
func newest(p string) time.Time {
	latest := fsx.ModTime(p)
	_ = filepath.WalkDir(p, func(q string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil && fi.ModTime().After(latest) {
				latest = fi.ModTime()
			}
		}
		return nil
	})
	return latest
}

// AIToolData finds old chat transcripts and scratch files left by AI coding tools.
// Settings, credentials and memory files are never included.
func AIToolData(sessionDays, scratchDays int) []Item {
	var out []Item
	// Claude Code: one transcript (and a sidecar folder) per session, per project.
	out = append(out, olderThan(sessionDays, "Claude Code session", func(n string) bool { return uuidName.MatchString(n) },
		".claude/projects/*/*")...)
	out = append(out, olderThan(scratchDays, "Claude Code scratch", nil,
		".claude/file-history/*", ".claude/shell-snapshots/*", ".claude/paste-cache/*", ".claude/debug/*", ".claude/todos/*")...)
	// Codex: sessions/YYYY/MM/DD
	out = append(out, olderThan(sessionDays, "Codex sessions", nil, ".codex/sessions/*/*/*")...)
	// Gemini CLI keeps chats under tmp/<project hash>.
	out = append(out, olderThan(sessionDays, "Gemini CLI chats", nil, ".gemini/tmp/*")...)
	return withoutExcluded(out)
}

var editors = []string{"Code", "Code - Insiders", "Cursor", "Windsurf", "VSCodium", "Antigravity"}

// EditorOrphans finds VS Code-style workspace state for folders that no longer exist.
func EditorOrphans() []Item {
	var out []Item
	for _, ed := range editors {
		dirs, _ := filepath.Glob(fsx.H("Library/Application Support/" + ed + "/User/workspaceStorage/*"))
		for _, d := range dirs {
			b, err := os.ReadFile(filepath.Join(d, "workspace.json"))
			if err != nil {
				continue
			}
			var w struct{ Folder, Workspace string }
			if json.Unmarshal(b, &w) != nil {
				continue
			}
			target := w.Folder
			if target == "" {
				target = w.Workspace
			}
			u, err := url.Parse(target)
			if err != nil || u.Scheme != "file" || u.Path == "" {
				continue // remote and virtual workspaces are left alone
			}
			if !fsx.Exists(u.Path) {
				out = append(out, Item{Path: d, Note: ed + " state for " + fsx.Pretty(u.Path) + " (folder is gone)"})
			}
		}
	}
	return withoutExcluded(out)
}

// AgentWorktrees finds git worktrees created by AI agents that are clean (no uncommitted or
// untracked changes) and idle for `days`. Removing one keeps its branch and commits.
func AgentWorktrees(roots []string, days int) []Item {
	var cands []string
	for _, pat := range []string{".claude-worktrees/*/*", ".codex/worktrees/*/*", ".cursor/worktrees/*/*"} {
		ms, _ := filepath.Glob(fsx.H(pat))
		cands = append(cands, ms...)
	}
	for _, r := range roots {
		for _, pat := range []string{"*/.claude/worktrees/*", "*/*/.claude/worktrees/*", "*/.worktrees/*", "*/*/.worktrees/*"} {
			ms, _ := filepath.Glob(filepath.Join(r, pat))
			cands = append(cands, ms...)
		}
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	items := make([]*Item, len(cands))
	Parallel(len(cands), func(i int) {
		wt := cands[i]
		b, err := os.ReadFile(filepath.Join(wt, ".git"))
		if err != nil || !strings.HasPrefix(string(b), "gitdir:") {
			return // not a linked worktree
		}
		gitdir := strings.TrimSpace(strings.TrimPrefix(string(b), "gitdir:"))
		j := strings.Index(gitdir, "/.git/worktrees/")
		if j < 0 {
			return
		}
		main := gitdir[:j]
		if st, err := sh.Quiet("git", "-C", wt, "status", "--porcelain"); err != nil || strings.TrimSpace(st) != "" {
			return // uncommitted work: leave it
		}
		if lastActivity(wt, cutoff).After(cutoff) {
			return
		}
		items[i] = &Item{Path: wt, Cmd: []string{"git", "-C", main, "worktree", "remove", wt},
			Note: "clean worktree of " + fsx.Pretty(main) + "; its branch is kept"}
	})
	var out []Item
	for _, it := range items {
		if it != nil {
			out = append(out, *it)
		}
	}
	return withoutExcluded(out)
}

// findRepos lists git repositories (folders with a .git directory) under roots.
func findRepos(roots []string) []string {
	var repos []string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() && p != root {
					return fs.SkipDir
				}
				return nil
			}
			if !d.IsDir() || p == root {
				return nil
			}
			name := d.Name()
			if name == ".git" {
				repos = append(repos, filepath.Dir(p))
				return fs.SkipDir
			}
			if p == fsx.H("Library") || isArtifact(name) || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".app") {
				return fs.SkipDir
			}
			return nil
		})
	}
	return repos
}

var protectedBranches = map[string]bool{"main": true, "master": true, "develop": true, "development": true, "dev": true,
	"trunk": true, "staging": true, "stage": true, "production": true, "prod": true, "release": true, "qa": true, "test": true}

func defaultBranch(repo string) string {
	if ref := sh.Line("git", "-C", repo, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); ref != "" {
		return strings.TrimPrefix(ref, "refs/remotes/origin/")
	}
	for _, b := range []string{"main", "master", "trunk", "develop"} {
		if _, err := sh.Quiet("git", "-C", repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+b); err == nil {
			return b
		}
	}
	return ""
}

// MergedBranches finds local branches already merged into the default branch whose last
// commit is older than `days`. `git branch -d` refuses anything that isn't merged.
func MergedBranches(roots []string, days int) []Item {
	repos := findRepos(roots)
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	items := make([]*Item, len(repos))
	Parallel(len(repos), func(i int) {
		repo := repos[i]
		base := defaultBranch(repo)
		if base == "" {
			return
		}
		current := sh.Line("git", "-C", repo, "branch", "--show-current")
		out, err := sh.Quiet("git", "-C", repo, "for-each-ref", "--merged", base, "--format=%(refname:short) %(committerdate:unix)", "refs/heads")
		if err != nil {
			return
		}
		var names []string
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			f := strings.Fields(line)
			if len(f) != 2 {
				continue
			}
			ts, _ := strconv.ParseInt(f[1], 10, 64)
			if f[0] == base || f[0] == current || protectedBranches[f[0]] || ts > cutoff {
				continue
			}
			names = append(names, f[0])
		}
		if len(names) == 0 {
			return
		}
		sort.Strings(names)
		preview := names
		if len(preview) > 4 {
			preview = append(append([]string(nil), names[:4]...), "…")
		}
		items[i] = &Item{Path: repo, NoSize: true, Cmd: append([]string{"git", "-C", repo, "branch", "-d"}, names...),
			Note: strconv.Itoa(len(names)) + " merged into " + base + ": " + strings.Join(preview, ", ")}
	})
	var res []Item
	for _, it := range items {
		if it != nil {
			res = append(res, *it)
		}
	}
	return res
}
