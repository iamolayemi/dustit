package doctor

import (
	"encoding/json"
	"fmt"
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
