package finder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/sh"
)

// Scopes of a launchd item.
const (
	ScopeUser      = "you"
	ScopeAllAgents = "all users"
	ScopeSystem    = "system"
)

// StartupItem is a launch agent or daemon: something that starts in the background.
type StartupItem struct {
	Label    string `json:"label"`
	Plist    string `json:"plist"`
	Program  string `json:"program"`
	Owner    string `json:"owner,omitempty"`
	Scope    string `json:"scope"`
	Loaded   bool   `json:"loaded"`
	Disabled bool   `json:"disabled"`
	Orphaned bool   `json:"orphaned"`
	Reason   string `json:"reason,omitempty"`
}

func startupDirs() map[string]string {
	return map[string]string{
		fsx.H("Library/LaunchAgents"): ScopeUser,
		"/Library/LaunchAgents":       ScopeAllAgents,
		"/Library/LaunchDaemons":      ScopeSystem,
	}
}

var disabledLine = regexp.MustCompile(`"([^"]+)" => (disabled|true)`)

func disabledLabels(domain string) map[string]bool {
	out := map[string]bool{}
	s, _ := sh.Quiet("launchctl", "print-disabled", domain)
	for _, m := range disabledLine.FindAllStringSubmatch(s, -1) {
		out[m[1]] = true
	}
	return out
}

// UserDomain is the launchd domain of the logged-in user.
func UserDomain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

// StartupItems lists every launch agent and daemon, orphans first.
func StartupItems() []StartupItem {
	loaded := map[string]bool{}
	if s, err := sh.Quiet("launchctl", "list"); err == nil {
		for _, line := range strings.Split(s, "\n") {
			if f := strings.Fields(line); len(f) == 3 {
				loaded[f[2]] = true
			}
		}
	}
	disabledUser, disabledSys := disabledLabels(UserDomain()), disabledLabels("system")
	names := installedNames()

	var items []StartupItem
	for dir, scope := range startupDirs() {
		files, _ := filepath.Glob(filepath.Join(dir, "*.plist"))
		for _, f := range files {
			items = append(items, StartupItem{Plist: f, Scope: scope})
		}
	}
	Parallel(len(items), func(i int) {
		it := &items[i]
		var p struct {
			Label            string
			Program          string
			ProgramArguments []string
		}
		if js, err := sh.Quiet("plutil", "-convert", "json", "-o", "-", it.Plist); err == nil {
			_ = json.Unmarshal([]byte(js), &p)
		}
		it.Label = p.Label
		if it.Label == "" {
			it.Label = strings.TrimSuffix(filepath.Base(it.Plist), ".plist")
		}
		it.Program = p.Program
		if it.Program == "" && len(p.ProgramArguments) > 0 {
			it.Program = p.ProgramArguments[0]
		}
		if svc := strings.TrimPrefix(strings.TrimPrefix(it.Label, "homebrew.mxcl."), "sh.brew."); svc != it.Label {
			it.Owner = "Homebrew service " + svc
		} else if i := strings.Index(it.Program, ".app/"); i >= 0 {
			it.Owner = strings.TrimSuffix(filepath.Base(it.Program[:i+4]), ".app")
		} else if o, ok := ownerOf(it.Label, names); ok {
			it.Owner = o
		}
		if filepath.IsAbs(it.Program) && !fsx.Exists(it.Program) {
			it.Orphaned, it.Reason = true, "its program is gone: "+fsx.Pretty(it.Program)
		}
		if it.Scope == ScopeSystem {
			it.Loaded = sh.Has("launchctl") && runOK("launchctl", "print", "system/"+it.Label)
			it.Disabled = disabledSys[it.Label]
		} else {
			it.Loaded = loaded[it.Label]
			it.Disabled = disabledUser[it.Label]
		}
	})
	rank := map[string]int{ScopeUser: 0, ScopeAllAgents: 1, ScopeSystem: 2}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Orphaned != b.Orphaned {
			return a.Orphaned
		}
		if rank[a.Scope] != rank[b.Scope] {
			return rank[a.Scope] < rank[b.Scope]
		}
		return a.Label < b.Label
	})
	return items
}

func runOK(args ...string) bool {
	_, err := sh.Quiet(args...)
	return err == nil
}
