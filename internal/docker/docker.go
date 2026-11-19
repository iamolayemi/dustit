// Package docker inspects and prunes Docker Desktop's disk image.
package docker

import (
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/sh"
)

// RawPath is Docker Desktop's VM disk image.
func RawPath() string {
	return fsx.H("Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw")
}

// RawSizes returns (space used on disk, maximum size) of the sparse disk image.
func RawSizes() (used, limit int64) {
	if !fsx.MayRead(RawPath()) {
		return 0, 0
	}
	var st syscall.Stat_t
	if syscall.Lstat(RawPath(), &st) != nil {
		return 0, 0
	}
	return st.Blocks * 512, st.Size
}

// Installed reports whether the docker CLI is available.
func Installed() bool { return sh.Has("docker") }

// Running reports whether the Docker engine answers.
func Running() bool {
	_, err := sh.Quiet("docker", "info", "--format", "{{.ServerVersion}}")
	return err == nil
}

// Start opens Docker Desktop and waits up to `wait` for the engine.
func Start(wait time.Duration) bool {
	if err := exec.Command("open", "-a", "Docker").Run(); err != nil {
		return false
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if Running() {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

// Step is one prune command.
type Step struct {
	Question string
	Args     []string
}

// PruneSteps are the safe prune commands (volumes are separate: they hold data).
var PruneSteps = []Step{
	{"Remove stopped containers?", []string{"docker", "container", "prune", "-f"}},
	{"Remove images not used by any container? (re-pulled when needed)", []string{"docker", "image", "prune", "-a", "-f"}},
	{"Remove all build cache? (next builds are slower)", []string{"docker", "builder", "prune", "-a", "-f"}},
	{"Remove unused networks?", []string{"docker", "network", "prune", "-f"}},
}

// UnusedVolumes lists volumes no container references.
func UnusedVolumes() []string {
	out, _ := sh.Quiet("docker", "volume", "ls", "-q", "--filter", "dangling=true")
	return strings.Fields(out)
}

// WaitForShrink polls the disk image until it stops shrinking (Docker returns freed
// blocks to macOS shortly after a prune) and returns the final on-disk size.
func WaitForShrink(from int64, tick func()) int64 {
	last, stable := from, 0
	for i := 0; i < 45 && stable < 5; i++ {
		time.Sleep(2 * time.Second)
		if u, _ := RawSizes(); u < last {
			last, stable = u, 0
		} else {
			stable++
		}
		if tick != nil {
			tick()
		}
	}
	u, _ := RawSizes()
	return u
}
