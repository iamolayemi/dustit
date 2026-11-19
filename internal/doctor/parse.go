package doctor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// parseLoadAvg reads `sysctl -n vm.loadavg` output: "{ 26.54 20.43 18.01 }".
func parseLoadAvg(s string) (one, five, fifteen float64, err error) {
	f := strings.Fields(strings.Trim(strings.TrimSpace(s), "{}"))
	if len(f) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected loadavg %q", s)
	}
	vals := make([]float64, 3)
	for i := range vals {
		if vals[i], err = strconv.ParseFloat(f[i], 64); err != nil {
			return 0, 0, 0, err
		}
	}
	return vals[0], vals[1], vals[2], nil
}

// parseSwap reads `sysctl -n vm.swapusage`: "total = 13312.00M  used = 12288.19M  free = 1023.81M  (encrypted)".
func parseSwap(s string) (total, used int64, err error) {
	get := func(key string) (int64, error) {
		i := strings.Index(s, key+" = ")
		if i < 0 {
			return 0, fmt.Errorf("no %s in %q", key, s)
		}
		v := strings.Fields(s[i+len(key)+3:])[0]
		mult := 1.0
		switch v[len(v)-1] {
		case 'K':
			mult = 1 << 10
		case 'M':
			mult = 1 << 20
		case 'G':
			mult = 1 << 30
		}
		f, err := strconv.ParseFloat(strings.TrimRight(v, "KMG"), 64)
		return int64(f * mult), err
	}
	if total, err = get("total"); err != nil {
		return
	}
	used, err = get("used")
	return
}

// parseEtime reads ps elapsed time: [[dd-]hh:]mm:ss.
func parseEtime(s string) time.Duration {
	s = strings.TrimSpace(s)
	var days int
	if i := strings.Index(s, "-"); i >= 0 {
		days, _ = strconv.Atoi(s[:i])
		s = s[i+1:]
	}
	parts := strings.Split(s, ":")
	secs := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		secs = secs*60 + n
	}
	return time.Duration(days)*24*time.Hour + time.Duration(secs)*time.Second
}

// Process is one running process.
type Process struct {
	PID     int
	CPU     float64 // percent of one core
	RSS     int64   // bytes
	Elapsed time.Duration
	Path    string
}

// Name is a readable name: the .app it belongs to, or the binary name.
func (p Process) Name() string { return appName(p.Path) }

func appName(path string) string {
	if i := strings.Index(path, ".app/"); i >= 0 {
		return filepath.Base(path[:i])
	}
	return filepath.Base(path)
}

// parsePS reads `ps -Ao pid=,pcpu=,rss=,etime=,comm=` output.
func parsePS(out string) []Process {
	var ps []Process
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		pid, err1 := strconv.Atoi(f[0])
		cpu, err2 := strconv.ParseFloat(f[1], 64)
		rss, err3 := strconv.ParseInt(f[2], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		// the command may contain spaces: it's everything after the 4th field
		rest := line
		for i := 0; i < 4; i++ {
			rest = strings.TrimLeft(rest, " ")
			rest = rest[strings.IndexByte(rest, ' ')+1:]
		}
		ps = append(ps, Process{PID: pid, CPU: cpu, RSS: rss * 1024, Elapsed: parseEtime(f[3]), Path: strings.TrimSpace(rest)})
	}
	return ps
}

// AppUsage is the combined usage of every process of one app.
type AppUsage struct {
	Name  string
	CPU   float64
	RSS   int64
	Count int
}

func groupByApp(ps []Process) []AppUsage {
	m := map[string]*AppUsage{}
	var order []string
	for _, p := range ps {
		n := p.Name()
		a, ok := m[n]
		if !ok {
			a = &AppUsage{Name: n}
			m[n] = a
			order = append(order, n)
		}
		a.CPU += p.CPU
		a.RSS += p.RSS
		a.Count++
	}
	out := make([]AppUsage, 0, len(order))
	for _, n := range order {
		out = append(out, *m[n])
	}
	return out
}

// brewServiceFor returns the Homebrew formula a process binary belongs to, if any.
func brewServiceFor(path string, services []string) string {
	for _, s := range services {
		if strings.Contains(path, "/opt/"+s+"/") || strings.Contains(path, "/Cellar/"+s+"/") {
			return s
		}
	}
	return ""
}
