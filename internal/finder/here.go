package finder

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/sh"
)

// Project is everything one project uses on disk.
type Project struct {
	Root      string
	Artifacts []Item // dependency and build folders, at any depth
	GitSize   int64
	Branches  *Item  // merged branches (a git branch -d command), if any
	Worktrees []Item // linked worktrees
	Compose   string // Docker Compose project name
	Docker    *DockerUse
}

// DockerUse is a project's footprint in Docker (Compose labels).
type DockerUse struct {
	Containers []string
	Images     []string
	ImageBytes int64
	Volumes    []string
}

// ProjectRoot is the git top level of dir, or dir itself.
func ProjectRoot(dir string) string {
	if top := sh.Line("git", "-C", dir, "rev-parse", "--show-toplevel"); filepath.IsAbs(top) {
		return top
	}
	return dir
}

var composeName = regexp.MustCompile(`[^a-z0-9_-]+`)

// ComposeProject is Docker Compose's default project name for a folder.
func ComposeProject(root string) string {
	return composeName.ReplaceAllString(strings.ToLower(filepath.Base(root)), "")
}

// ProjectArtifacts finds dependency and build folders anywhere inside root.
func ProjectArtifacts(root string) []Item {
	var out []Item
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
			return fs.SkipDir
		}
		if markers, ok := artifactRules[name]; ok && hasMarker(filepath.Dir(p), name, markers) {
			out = append(out, Item{Path: p, Note: "recreate with the project's install/build command"})
			return fs.SkipDir
		}
		return nil
	})
	out = withoutExcluded(out)
	Measure(out)
	SortBySize(out)
	return out
}

// InspectProject gathers a project's dependencies, git and Docker footprint.
func InspectProject(dir string) Project {
	root := ProjectRoot(dir)
	p := Project{Root: root, Compose: ComposeProject(root)}
	p.Artifacts = ProjectArtifacts(root)
	if fsx.IsDir(filepath.Join(root, ".git")) {
		p.GitSize = fsx.DiskUsage(filepath.Join(root, ".git")).Bytes
		if b := MergedBranches([]string{filepath.Dir(root)}, 0); len(b) > 0 {
			for i := range b {
				if b[i].Path == root {
					p.Branches = &b[i]
				}
			}
		}
		if out, err := sh.Quiet("git", "-C", root, "worktree", "list", "--porcelain"); err == nil {
			for _, line := range strings.Split(out, "\n") {
				if wt, ok := strings.CutPrefix(line, "worktree "); ok && wt != root {
					p.Worktrees = append(p.Worktrees, Item{Path: wt})
				}
			}
			Measure(p.Worktrees)
		}
	}
	p.Docker = composeUse(p.Compose)
	return p
}

func composeUse(project string) *DockerUse {
	if !sh.Has("docker") {
		return nil
	}
	if _, err := sh.Quiet("docker", "info", "--format", "{{.ServerVersion}}"); err != nil {
		return nil
	}
	label := "label=com.docker.compose.project=" + project
	u := &DockerUse{}
	if s, err := sh.Quiet("docker", "ps", "-a", "--filter", label, "--format", "{{.Names}}\t{{.Image}}"); err == nil {
		seen := map[string]bool{}
		for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
			if name, img, ok := strings.Cut(line, "\t"); ok {
				u.Containers = append(u.Containers, name)
				if !seen[img] {
					seen[img] = true
					u.Images = append(u.Images, img)
				}
			}
		}
	}
	if s, err := sh.Quiet("docker", "volume", "ls", "-q", "--filter", label); err == nil {
		u.Volumes = strings.Fields(s)
	}
	for _, img := range u.Images {
		if s := sh.Line("docker", "image", "inspect", "--format", "{{.Size}}", img); s != "" {
			n, _ := strconv.ParseInt(s, 10, 64)
			u.ImageBytes += n
		}
	}
	if len(u.Containers)+len(u.Volumes) == 0 {
		return nil
	}
	return u
}
