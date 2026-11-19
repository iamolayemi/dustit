// Package doctor diagnoses why a Mac feels slow. Checks only read system state;
// the few safe fixes run only when the caller invokes Fix.Run.
package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/sh"
)

// Severity of a finding.
type Severity int

const (
	OK Severity = iota
	Info
	Warn
	Bad
)

// Fix is a safe, specific action that addresses a finding.
type Fix struct {
	Description string
	Run         func() error
}

// Finding is the result of one check.
type Finding struct {
	Check    string
	Severity Severity
	Summary  string
	Short    string // a few words for compact views ("6× overloaded"); Summary when empty
	Details  []string
	Advice   []string
	Fixes    []Fix
}

// Human formats bytes; set by the caller so output matches the rest of the CLI.
type Human func(int64) string

// Run executes every check concurrently and returns findings, worst first.
func Run(human Human) []Finding {
	var ps []Process
	var services []brewService
	var wg0 sync.WaitGroup
	wg0.Add(2)
	go func() { defer wg0.Done(); ps = processes() }()
	go func() { defer wg0.Done(); services = brewServices() }()
	wg0.Wait()
	checks := []func() Finding{
		func() Finding { return checkLoad(ps, services) },
		func() Finding { return checkMemory(ps, human) },
		func() Finding { return checkDisk(human) },
		checkThermal,
		func() Finding { return checkUptime() },
		checkStartupAgents,
		func() Finding { return checkServices(services) },
		checkSnapshots,
		checkBattery,
		checkUpdates,
		func() Finding {
			ram, _ := strconv.ParseInt(sysctl("hw.memsize"), 10, 64)
			return checkDockerMemory(ram, human)
		},
		func() Finding { return checkSpotlight(ps, sh.Line("mdutil", "-s", "/")) },
		func() Finding {
			ram, _ := strconv.ParseInt(sysctl("hw.memsize"), 10, 64)
			return checkBrowsers(ps, ram, human)
		},
	}
	out := make([]Finding, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		go func(i int, c func() Finding) { defer wg.Done(); out[i] = c() }(i, c)
	}
	wg.Wait()
	sort.SliceStable(out, func(i, j int) bool { return out[i].Severity > out[j].Severity })
	return out
}

// Quick runs only the fast CPU and memory checks (for live views).
func Quick(human Human) []Finding {
	ps := processes()
	return []Finding{checkLoad(ps, nil), checkMemory(ps, human)}
}

func sysctl(name string) string { return sh.Line("sysctl", "-n", name) }

func cores() int {
	n, _ := strconv.Atoi(sysctl("hw.ncpu"))
	if n < 1 {
		n = 1
	}
	return n
}

func processes() []Process {
	out, err := sh.Quiet("ps", "-Ao", "pid=,pcpu=,rss=,etime=,comm=")
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var ps []Process
	for _, p := range parsePS(out) {
		if p.PID != self && filepath.Base(p.Path) != "ps" {
			ps = append(ps, p)
		}
	}
	return ps
}

type brewService struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	PID    int    `json:"pid"`
}

func brewServices() []brewService {
	if !sh.Has("brew") {
		return nil
	}
	out, err := sh.Quiet("brew", "services", "list", "--json")
	if err != nil {
		return nil
	}
	var all []brewService
	if json.Unmarshal([]byte(out), &all) != nil {
		return nil
	}
	var running []brewService
	for _, s := range all {
		if s.Status == "started" {
			running = append(running, s)
		}
	}
	return running
}

// Well-known processes and what their high CPU usually means.
var processHints = map[string]string{
	"mds_stores":         "Spotlight is indexing; it finishes on its own. If it never stops: sudo mdutil -E /",
	"mds":                "Spotlight is indexing; it finishes on its own.",
	"mdworker_shared":    "Spotlight is reading new files; it finishes on its own.",
	"kernel_task":        "macOS is slowing the CPU to cool down. Check ventilation and what else is busy.",
	"WindowServer":       "Draws the screen. Extra displays, many open windows and transparency effects raise it.",
	"backupd":            "Time Machine is backing up.",
	"photoanalysisd":     "Photos is analysing your library; it runs while the Mac is idle and plugged in.",
	"softwareupdated":    "macOS is downloading an update.",
	"com.docker.backend": "Docker Desktop. Its VM keeps using CPU and memory while it runs; quit it when idle.",
}

func checkLoad(ps []Process, services []brewService) Finding {
	f := Finding{Check: "CPU load"}
	_, five, _, err := parseLoadAvg(sysctl("vm.loadavg"))
	if err != nil {
		f.Summary = "couldn't read load average"
		return f
	}
	n := cores()
	ratio := five / float64(n)
	f.Summary = fmt.Sprintf("%.1f tasks waiting on %d cores (5-min average)", five, n)
	f.Short = fmt.Sprintf("%.0f%% busy", ratio*100)
	if ratio > 1 {
		f.Short = fmt.Sprintf("%.0f× overloaded", ratio)
	}
	switch {
	case ratio > 2:
		f.Severity = Bad
		f.Summary += fmt.Sprintf(" — %.0f× more work than the CPU can run", ratio)
	case ratio > 1:
		f.Severity = Warn
		f.Summary += " — busier than the CPU can keep up with"
	}

	sorted := append([]Process(nil), ps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CPU > sorted[j].CPU })
	names := make([]string, 0, len(services))
	byPID := map[int]string{}
	for _, s := range services {
		names = append(names, s.Name)
		byPID[s.PID] = s.Name
	}
	for i, p := range sorted {
		if i == 5 || p.CPU < 20 {
			break
		}
		line := fmt.Sprintf("%5.0f%%  %-28s open for %s", p.CPU, p.Name(), roundDur(p.Elapsed))
		f.Details = append(f.Details, line)

		if hint, ok := processHints[filepath.Base(p.Path)]; ok {
			f.Advice = append(f.Advice, p.Name()+": "+hint)
		}
		// A process that has been burning CPU for over an hour is rarely doing useful work.
		if p.CPU >= 50 && p.Elapsed > time.Hour {
			if f.Severity < Warn {
				f.Severity = Warn
			}
			svc := byPID[p.PID]
			if svc == "" {
				svc = brewServiceFor(p.Path, names)
			}
			if svc != "" {
				name := svc
				f.Advice = append(f.Advice, fmt.Sprintf("%s is using %.0f%% CPU and has been running %s. A service this busy is often stuck on a long query or leftover job. Restart it: brew services restart %s",
					p.Name(), p.CPU, roundDur(p.Elapsed), name))
				f.Fixes = append(f.Fixes, Fix{
					Description: "Restart the Homebrew service " + name,
					Run:         func() error { return sh.Shown("brew", "services", "restart", name) },
				})
			} else if _, known := processHints[filepath.Base(p.Path)]; !known {
				f.Advice = append(f.Advice, fmt.Sprintf("%s (pid %d) is using %.0f%% CPU and has been open %s. If you aren't using it, quit it; reopening often fixes runaway CPU.",
					p.Name(), p.PID, p.CPU, roundDur(p.Elapsed)))
			}
		}
	}
	return f
}

func checkMemory(ps []Process, human Human) Finding {
	f := Finding{Check: "Memory"}
	ram, _ := strconv.ParseInt(sysctl("hw.memsize"), 10, 64)
	level, _ := strconv.Atoi(sysctl("kern.memorystatus_vm_pressure_level"))
	_, swapUsed, _ := parseSwap(sysctl("vm.swapusage"))

	f.Summary = fmt.Sprintf("%s RAM, %s swap in use", human(ram), human(swapUsed))
	f.Short = human(swapUsed) + " swapped to disk"
	switch {
	case level >= 4:
		f.Severity = Bad
		f.Summary += " — memory pressure is critical"
	case level == 2:
		f.Severity = Warn
		f.Summary += " — memory pressure is elevated"
	case ram > 0 && swapUsed > ram/4:
		f.Severity = Warn
		f.Summary += " — heavy swapping to disk"
	}

	apps := groupByApp(ps)
	sort.Slice(apps, func(i, j int) bool { return apps[i].RSS > apps[j].RSS })
	for i, a := range apps {
		if i == 6 || a.RSS < 200<<20 {
			break
		}
		procs := ""
		if a.Count > 1 {
			procs = fmt.Sprintf(" (%d processes)", a.Count)
		}
		f.Details = append(f.Details, fmt.Sprintf("%9s  %s%s", human(a.RSS), a.Name, procs))
	}
	if f.Severity >= Warn {
		f.Advice = append(f.Advice,
			"Quit apps you aren't using, starting with the largest above; browsers with many tabs add up fast.",
			"Restarting the Mac clears swap that has built up over days.")
	}
	return f
}

func checkDisk(human Human) Finding {
	f := Finding{Check: "Disk space"}
	var s syscall.Statfs_t
	if syscall.Statfs(fsx.Home, &s) != nil {
		f.Summary = "couldn't read disk usage"
		return f
	}
	free := int64(s.Bavail) * int64(s.Bsize)
	total := int64(s.Blocks) * int64(s.Bsize)
	pct := float64(free) / float64(total) * 100
	f.Summary = fmt.Sprintf("%s free (%.0f%%)", human(free), pct)
	f.Short = fmt.Sprintf("%.0f%% free", pct)
	switch {
	case pct < 10:
		f.Severity = Bad
		f.Summary += " — macOS slows down and can't swap well this full"
	case pct < 20:
		f.Severity = Warn
		f.Summary += " — getting full"
	}
	if f.Severity >= Warn {
		f.Advice = append(f.Advice, "Free space: dustit scan, then dustit clean")
	}
	return f
}

func checkThermal() Finding {
	f := Finding{Check: "Temperature"}
	out, err := sh.Quiet("pmset", "-g", "therm")
	if err != nil {
		f.Summary = "couldn't read thermal state"
		return f
	}
	f.Summary = "no throttling"
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == "CPU_Speed_Limit" {
			if n, _ := strconv.Atoi(strings.TrimSpace(v)); n > 0 && n < 100 {
				f.Severity = Bad
				f.Summary = fmt.Sprintf("CPU limited to %d%% speed to cool down", n)
				f.Short = fmt.Sprintf("slowed to %d%% to cool down", n)
				f.Advice = append(f.Advice, "Let the Mac cool: check vents, don't run it on a soft surface, and reduce what's using CPU.")
			}
		}
	}
	return f
}

func checkUptime() Finding {
	f := Finding{Check: "Uptime"}
	raw := sysctl("kern.boottime") // "{ sec = 1762000000, usec = 0 } ..."
	i := strings.Index(raw, "sec = ")
	if i < 0 {
		f.Summary = "unknown"
		return f
	}
	secStr := strings.TrimRight(strings.Fields(raw[i+6:])[0], ",")
	sec, _ := strconv.ParseInt(secStr, 10, 64)
	up := time.Since(time.Unix(sec, 0))
	f.Summary = "running for " + roundDur(up)
	f.Short = "no restart in " + roundDur(up)
	if up > 7*24*time.Hour {
		f.Severity = Info
		f.Advice = append(f.Advice, "A restart every week or so clears built-up swap and stuck background processes.")
	}
	return f
}

// launch agents/daemons whose program no longer exists.
func brokenAgents(dir string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, "*.plist"))
	var broken []string
	for _, p := range files {
		prog := sh.Line("plutil", "-extract", "Program", "raw", "-o", "-", p)
		if !filepath.IsAbs(prog) {
			prog = sh.Line("plutil", "-extract", "ProgramArguments.0", "raw", "-o", "-", p)
		}
		if filepath.IsAbs(prog) && !fsx.Exists(prog) {
			broken = append(broken, p)
		}
	}
	return broken
}

func checkStartupAgents() Finding {
	f := Finding{Check: "Startup items"}
	userDir := fsx.H("Library/LaunchAgents")
	dirs := []string{userDir, "/Library/LaunchAgents", "/Library/LaunchDaemons"}
	total := 0
	var broken []string
	for _, d := range dirs {
		files, _ := filepath.Glob(filepath.Join(d, "*.plist"))
		total += len(files)
		broken = append(broken, brokenAgents(d)...)
	}
	f.Summary = fmt.Sprintf("%d launch agents and daemons", total)
	if len(broken) == 0 {
		return f
	}
	f.Severity = Warn
	f.Summary += fmt.Sprintf(", %d point at programs that no longer exist", len(broken))
	f.Short = fmt.Sprintf("%d left behind by removed apps", len(broken))
	for _, b := range broken {
		f.Details = append(f.Details, fsx.Pretty(b))
		if filepath.Dir(b) == userDir {
			plist := b
			f.Fixes = append(f.Fixes, Fix{
				Description: "Unload and move to Trash " + fsx.Pretty(plist),
				Run: func() error {
					_ = sh.Shown("launchctl", "bootout", "gui/"+strconv.Itoa(os.Getuid()), plist)
					return fsx.Remove(plist, true)
				},
			})
		} else {
			f.Advice = append(f.Advice, "Needs an admin password to remove: sudo launchctl bootout system "+b+" && sudo rm "+b)
		}
	}
	f.Advice = append(f.Advice, "Also review System Settings → General → Login Items & Extensions.")
	return f
}

func checkServices(services []brewService) Finding {
	f := Finding{Check: "Background services"}
	if len(services) == 0 {
		f.Summary = "no Homebrew services running"
		return f
	}
	var names []string
	for _, s := range services {
		names = append(names, s.Name)
	}
	f.Summary = fmt.Sprintf("%d Homebrew services running: %s", len(services), strings.Join(names, ", "))
	f.Short = fmt.Sprintf("%d services always on", len(services))
	if len(services) >= 3 {
		f.Severity = Info
		f.Advice = append(f.Advice, "Each one uses memory all the time. Stop the ones you don't need today: brew services stop <name>")
	}
	return f
}

func checkSnapshots() Finding {
	f := Finding{Check: "Time Machine snapshots"}
	out, err := sh.Quiet("tmutil", "listlocalsnapshots", "/")
	if err != nil {
		f.Summary = "couldn't list"
		return f
	}
	n := strings.Count(out, "com.apple.TimeMachine")
	f.Summary = fmt.Sprintf("%d local snapshots", n)
	if n > 0 {
		f.Severity = Info
		f.Advice = append(f.Advice, "They can hold space from deleted files. macOS removes them when needed; to remove now: tmutil thinlocalsnapshots / 999999999999 4")
	}
	return f
}

func roundDur(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		days := int(d.Hours()) / 24
		return fmt.Sprintf("%dd %dh", days, int(d.Hours())-days*24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
}
