package finder

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/sh"
)

// BigGitRepos finds repositories whose .git folder is at least minB. Cleaning runs
// `git worktree prune` and `git gc`, which repacks objects and drops loose, unreachable
// ones older than git's own two-week grace period (the reflog is left alone).
func BigGitRepos(roots []string, minB int64) []Item {
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
	items := make([]Item, len(repos))
	Parallel(len(repos), func(i int) {
		git := filepath.Join(repos[i], ".git")
		items[i] = Item{Path: git, Size: fsx.DiskUsage(git).Bytes,
			Cmd:  []string{"sh", "-c", `git -C "$1" worktree prune && git -C "$1" gc --quiet`, "sh", repos[i]},
			Note: "git gc (history is kept)"}
	})
	var out []Item
	for _, it := range items {
		if it.Size >= minB {
			out = append(out, it)
		}
	}
	return withoutExcluded(out)
}

// DevVMs lists local Kubernetes clusters and Linux VMs (minikube, kind, k3d, Colima, Lima).
// Removing one deletes the VM or cluster and everything inside it.
func DevVMs() []Item {
	var out []Item
	if sh.Has("minikube") {
		if s, err := sh.Quiet("minikube", "profile", "list", "-o", "json"); err == nil {
			var data struct {
				Valid []struct{ Name string } `json:"valid"`
			}
			if json.Unmarshal([]byte(s), &data) == nil {
				for _, p := range data.Valid {
					out = append(out, Item{Path: fsx.H(".minikube/machines/" + p.Name),
						Cmd: []string{"minikube", "delete", "-p", p.Name}, Note: "minikube cluster " + p.Name})
				}
			}
		}
	}
	for _, tool := range []struct{ bin, list, kind string }{
		{"kind", "kind get clusters", "kind"}, {"k3d", "k3d cluster list -o json", "k3d"},
	} {
		if !sh.Has(tool.bin) {
			continue
		}
		s, err := sh.Quiet("sh", "-c", tool.list)
		if err != nil {
			continue
		}
		var names []string
		if tool.kind == "k3d" {
			var cs []struct{ Name string }
			_ = json.Unmarshal([]byte(s), &cs)
			for _, c := range cs {
				names = append(names, c.Name)
			}
		} else {
			names = strings.Fields(s)
		}
		for _, n := range names {
			del := []string{"kind", "delete", "cluster", "--name", n}
			if tool.kind == "k3d" {
				del = []string{"k3d", "cluster", "delete", n}
			}
			out = append(out, Item{Path: tool.kind + " cluster " + n, Cmd: del,
				Note: tool.kind + " cluster (lives in Docker; size shows in `macclean docker`)"})
		}
	}
	if sh.Has("limactl") {
		if s, err := sh.Quiet("limactl", "list", "--json"); err == nil {
			sc := bufio.NewScanner(strings.NewReader(s))
			for sc.Scan() {
				var vm struct{ Name, Dir string }
				if json.Unmarshal(sc.Bytes(), &vm) == nil && vm.Name != "" && !strings.HasPrefix(vm.Name, "colima") {
					out = append(out, Item{Path: vm.Dir, Cmd: []string{"limactl", "delete", "-f", vm.Name}, Note: "Lima VM " + vm.Name})
				}
			}
		}
	}
	if sh.Has("colima") {
		if s, err := sh.Quiet("colima", "list", "--json"); err == nil {
			sc := bufio.NewScanner(strings.NewReader(s))
			for sc.Scan() {
				var vm struct{ Name string }
				if json.Unmarshal(sc.Bytes(), &vm) == nil && vm.Name != "" {
					dir := fsx.H(".colima/_lima/colima")
					if vm.Name != "default" {
						dir += "-" + vm.Name
					}
					out = append(out, Item{Path: dir, Cmd: []string{"colima", "delete", "-f", vm.Name}, Note: "Colima VM " + vm.Name})
				}
			}
		}
	}
	return withoutExcluded(out)
}

const sfDataless = 0x40000000 // SF_DATALESS: the file's contents live only in iCloud

// ICloudDownloads finds downloaded iCloud Drive files of at least minB that haven't
// changed in `days` days. Evicting removes the local copy; the file stays in iCloud and
// downloads again when opened.
func ICloudDownloads(days int, minB int64) []Item {
	root := fsx.H("Library/Mobile Documents")
	if !fsx.IsDir(root) || !sh.Has("brctl") {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	var out []Item
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && p != root {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		var st syscall.Stat_t
		if syscall.Lstat(p, &st) != nil || st.Flags&sfDataless != 0 {
			return nil
		}
		size := st.Blocks * 512
		if t := fsx.LastTouched(&st); size >= minB && t.Before(cutoff) {
			out = append(out, Item{Path: p, Size: size, Age: t, Cmd: []string{"brctl", "evict", p},
				Note: "stays in iCloud; last changed " + humanAgo(t)})
		}
		return nil
	})
	return withoutExcluded(out)
}

// GradleDaemonLogs are the logs Gradle daemons leave behind.
func GradleDaemonLogs() []Item {
	ms, _ := filepath.Glob(fsx.H(".gradle/daemon/*/*.log"))
	var out []Item
	for _, m := range ms {
		out = append(out, Item{Path: m})
	}
	return out
}
