package doctor

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/iamolayemi/macclean/internal/sh"
)

func checkBattery() Finding {
	f := Finding{Check: "Battery"}
	out, err := sh.Quiet("system_profiler", "SPPowerDataType", "-json")
	if err != nil {
		f.Summary = "couldn't read battery information"
		return f
	}
	var data struct {
		Items []struct {
			Health *struct {
				Cycles    int    `json:"sppower_battery_cycle_count"`
				Condition string `json:"sppower_battery_health"`
				MaxCap    string `json:"sppower_battery_health_maximum_capacity"`
			} `json:"sppower_battery_health_info"`
		} `json:"SPPowerDataType"`
	}
	if json.Unmarshal([]byte(out), &data) != nil {
		f.Summary = "couldn't read battery information"
		return f
	}
	for _, it := range data.Items {
		h := it.Health
		if h == nil {
			continue
		}
		f.Summary = fmt.Sprintf("%s condition, %s of original capacity, %d charge cycles", strings.ToLower(h.Condition), h.MaxCap, h.Cycles)
		f.Short = fmt.Sprintf("%s capacity · %d cycles", h.MaxCap, h.Cycles)
		capPct, _ := strconv.Atoi(strings.TrimSuffix(h.MaxCap, "%"))
		switch {
		case h.Condition != "" && h.Condition != "Good" && h.Condition != "Normal":
			f.Severity = Bad
		case capPct > 0 && capPct < 80:
			f.Severity = Warn
		}
		if f.Severity >= Warn {
			f.Advice = append(f.Advice,
				"A worn battery can make macOS limit performance under load, especially on battery power.",
				"Check System Settings → Battery → Battery Health, and consider a battery service.")
		}
		return f
	}
	f.Summary = "no battery (desktop Mac)"
	return f
}

func checkUpdates() Finding {
	f := Finding{Check: "macOS updates"}
	n, err := strconv.Atoi(sh.Line("defaults", "read", "/Library/Preferences/com.apple.SoftwareUpdate", "LastRecommendedUpdatesAvailable"))
	if err != nil {
		f.Summary = "unknown"
		return f
	}
	if n == 0 {
		f.Summary = "up to date"
		return f
	}
	f.Severity = Info
	f.Summary = fmt.Sprintf("%d recommended update(s) waiting", n)
	f.Short = fmt.Sprintf("%d waiting", n)
	f.Advice = append(f.Advice, "Updates often include performance and memory fixes: System Settings → General → Software Update.")
	return f
}

func checkDockerMemory(ram int64, human Human) Finding {
	f := Finding{Check: "Docker"}
	if !sh.Has("docker") {
		f.Summary = "not installed"
		return f
	}
	mem, err := strconv.ParseInt(sh.Line("docker", "info", "--format", "{{.MemTotal}}"), 10, 64)
	if err != nil || mem <= 0 {
		f.Summary = "not running"
		return f
	}
	f.Summary = fmt.Sprintf("Docker Desktop's VM has %s of your %s RAM", human(mem), human(ram))
	f.Short = human(mem) + " reserved by its VM"
	if ram > 0 && mem > ram/2 {
		f.Severity = Warn
		f.Advice = append(f.Advice,
			"The VM holds on to this memory while Docker runs. Lower it in Docker Desktop → Settings → Resources, or quit Docker when you're not using it.")
	} else {
		f.Severity = Info
		f.Advice = append(f.Advice, "Quit Docker Desktop when you're not using it to give that memory back.")
	}
	return f
}

var spotlightProcs = map[string]bool{"mds": true, "mds_stores": true, "mdworker": true, "mdworker_shared": true, "mdsync": true}

func checkSpotlight(ps []Process, status string) Finding {
	f := Finding{Check: "Spotlight"}
	if strings.Contains(status, "disabled") {
		f.Severity = Info
		f.Summary = "indexing is turned off for the startup disk"
		f.Advice = append(f.Advice, "Search, Mail and some apps rely on it. Turn it back on with: sudo mdutil -i on /")
		return f
	}
	var cpu float64
	for _, p := range ps {
		if spotlightProcs[filepath.Base(p.Path)] {
			cpu += p.CPU
		}
	}
	f.Summary = fmt.Sprintf("indexing on, using %.0f%% CPU", cpu)
	f.Short = fmt.Sprintf("indexing · %.0f%% CPU", cpu)
	switch {
	case cpu >= 80:
		f.Severity = Warn
		f.Summary += " — busy indexing"
		f.Advice = append(f.Advice,
			"Indexing after an update or many new files is normal and finishes on its own.",
			"If it stays this busy for hours, rebuild the index: sudo mdutil -E /",
			"Exclude big folders you never search (node_modules-heavy project folders, VM disks) in System Settings → Spotlight → Search Privacy.")
	case cpu >= 30:
		f.Severity = Info
		f.Summary += " — indexing new files"
	}
	return f
}

// browserApps maps process/app names to the browser they belong to.
var browserApps = map[string]string{
	"Google Chrome": "Chrome", "Google Chrome Helper": "Chrome", "Arc": "Arc", "Brave Browser": "Brave",
	"Microsoft Edge": "Edge", "Firefox": "Firefox", "Opera": "Opera", "Vivaldi": "Vivaldi", "Safari": "Safari",
	"com.apple.WebKit.WebContent": "Safari & WebKit apps", "com.apple.WebKit.GPU": "Safari & WebKit apps",
	"com.apple.WebKit.Networking": "Safari & WebKit apps", "Zen": "Zen", "Orion": "Orion",
}

func checkBrowsers(ps []Process, ram int64, human Human) Finding {
	f := Finding{Check: "Browsers"}
	type use struct {
		rss   int64
		count int
	}
	by := map[string]*use{}
	var order []string
	var total int64
	for _, p := range ps {
		b, ok := browserApps[p.Name()]
		if !ok {
			continue
		}
		if by[b] == nil {
			by[b] = &use{}
			order = append(order, b)
		}
		by[b].rss += p.RSS
		by[b].count++
		total += p.RSS
	}
	if total == 0 {
		f.Summary = "no browser running"
		return f
	}
	sort.Slice(order, func(i, j int) bool { return by[order[i]].rss > by[order[j]].rss })
	for _, b := range order {
		f.Details = append(f.Details, fmt.Sprintf("%9s  %s (%d processes)", human(by[b].rss), b, by[b].count))
	}
	f.Summary = fmt.Sprintf("using %s of memory", human(total))
	f.Short = human(total) + " of memory"
	if ram > 0 && total > ram/4 {
		f.Severity = Warn
		f.Summary += fmt.Sprintf(" — %.0f%% of your RAM", float64(total)/float64(ram)*100)
		f.Advice = append(f.Advice,
			"Each tab is its own process. Close tabs you aren't using, or let the browser sleep them:",
			"Chrome/Edge/Brave: Settings → Performance → Memory Saver. Arc: Settings → Auto Archive. Safari unloads background tabs itself.")
	}
	return f
}
