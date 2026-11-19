// Package privacy reads macOS security and privacy settings. It never changes anything:
// each check links to the System Settings pane where the user can change it.
package privacy

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/iamolayemi/macclean/internal/fsx"
	"github.com/iamolayemi/macclean/internal/sh"
)

// Status of a check.
type Status string

const (
	Good    Status = "good"
	Notice  Status = "notice" // worth knowing, a matter of preference
	Risk    Status = "risk"   // weakens protection
	Unknown Status = "unknown"
)

// Check is one setting.
type Check struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Summary string `json:"summary"`
	Advice  string `json:"advice,omitempty"`
	Pane    string `json:"pane"` // x-apple.systempreferences URL
}

const (
	panePrivacy  = "x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension"
	paneFirewall = "x-apple.systempreferences:com.apple.Network-Settings.extension?Firewall"
	paneUpdates  = "x-apple.systempreferences:com.apple.Software-Update-Settings.extension"
	paneLock     = "x-apple.systempreferences:com.apple.Lock-Screen-Settings.extension"
	paneSharing  = "x-apple.systempreferences:com.apple.Sharing-Settings.extension"
	paneAirDrop  = "x-apple.systempreferences:com.apple.AirDrop-Handoff-Settings.extension"
	paneUsers    = "x-apple.systempreferences:com.apple.Users-Groups-Settings.extension"
	paneAppleID  = "x-apple.systempreferences:com.apple.systempreferences.AppleIDSettings"
)

func out(args ...string) string {
	b, _ := exec.Command(args[0], args[1:]...).CombinedOutput()
	return strings.TrimSpace(string(b))
}

func defaultsRead(domain, key string) (string, bool) {
	v, err := sh.Quiet("defaults", "read", domain, key)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(v), true
}

// Run executes every check concurrently, in a stable order.
func Run() []Check {
	fns := []func() Check{fileVault, firewall, stealth, gatekeeper, sip, updates, screenLock,
		sharing, airDrop, findMy, autoLogin, guest, appPermissions}
	res := make([]Check, len(fns))
	var wg sync.WaitGroup
	for i, f := range fns {
		wg.Add(1)
		go func(i int, f func() Check) { defer wg.Done(); res[i] = f() }(i, f)
	}
	wg.Wait()
	var outList []Check
	for _, c := range res {
		if c.ID != "" {
			outList = append(outList, c)
		}
	}
	return outList
}

func fileVault() Check {
	c := Check{ID: "filevault", Name: "FileVault disk encryption", Pane: panePrivacy}
	switch out("fdesetup", "isactive") {
	case "true":
		c.Status, c.Summary = Good, "On"
	case "false":
		c.Status, c.Summary = Risk, "Off"
		c.Advice = "Anyone with the Mac can read its disk. Turn on FileVault in Privacy & Security."
	default:
		c.Status, c.Summary = Unknown, "Couldn't read"
	}
	return c
}

func firewall() Check {
	c := Check{ID: "firewall", Name: "Firewall", Pane: paneFirewall}
	s := out("/usr/libexec/ApplicationFirewall/socketfilterfw", "--getglobalstate")
	switch {
	case strings.Contains(s, "enabled"):
		c.Status, c.Summary = Good, "On"
	case strings.Contains(s, "disabled"):
		c.Status, c.Summary = Risk, "Off"
		c.Advice = "Blocks unwanted incoming connections, especially on public Wi-Fi."
	default:
		c.Status, c.Summary = Unknown, "Couldn't read"
	}
	return c
}

func stealth() Check {
	c := Check{ID: "stealth", Name: "Firewall stealth mode", Pane: paneFirewall}
	s := out("/usr/libexec/ApplicationFirewall/socketfilterfw", "--getstealthmode")
	switch {
	case strings.Contains(s, " on"):
		c.Status, c.Summary = Good, "On"
	case strings.Contains(s, " off"):
		c.Status, c.Summary = Notice, "Off"
		c.Advice = "With stealth mode the Mac doesn't answer pings or probes from the network. Useful on public Wi-Fi."
	default:
		return Check{}
	}
	return c
}

func gatekeeper() Check {
	c := Check{ID: "gatekeeper", Name: "Gatekeeper", Pane: panePrivacy}
	s := out("spctl", "--status")
	switch {
	case strings.Contains(s, "enabled"):
		c.Status, c.Summary = Good, "On"
	case strings.Contains(s, "disabled"):
		c.Status, c.Summary = Risk, "Off — any app can open without a check"
		c.Advice = "Turn it back on with: sudo spctl --master-enable"
	default:
		c.Status, c.Summary = Unknown, "Couldn't read"
	}
	return c
}

func sip() Check {
	c := Check{ID: "sip", Name: "System Integrity Protection", Pane: panePrivacy}
	s := out("csrutil", "status")
	switch {
	case strings.Contains(s, "enabled."):
		c.Status, c.Summary = Good, "On"
	case strings.Contains(s, "disabled"):
		c.Status, c.Summary = Risk, "Off"
		c.Advice = "Protects macOS's own files from changes. Re-enable it from Recovery: csrutil enable"
	default:
		c.Status, c.Summary = Unknown, "Couldn't read"
	}
	return c
}

func updates() Check {
	c := Check{ID: "updates", Name: "Automatic security updates", Pane: paneUpdates}
	const dom = "/Library/Preferences/com.apple.SoftwareUpdate"
	off := []string{}
	for key, label := range map[string]string{"AutomaticCheckEnabled": "checking", "CriticalUpdateInstall": "security responses",
		"ConfigDataInstall": "system data files", "AutomaticDownload": "downloading"} {
		if v, ok := defaultsRead(dom, key); ok && v == "0" {
			off = append(off, label)
		}
	}
	if len(off) == 0 {
		c.Status, c.Summary = Good, "On"
	} else {
		c.Status, c.Summary = Risk, "Off for "+strings.Join(off, ", ")
		c.Advice = "Security responses fix actively exploited bugs; keep them automatic."
	}
	if n, ok := defaultsRead(dom, "LastRecommendedUpdatesAvailable"); ok && n != "0" {
		c.Summary += " · " + n + " update(s) waiting"
		if c.Status == Good {
			c.Status = Notice
		}
	}
	return c
}

var lockDelay = regexp.MustCompile(`screenLock delay is (\d+) seconds`)

func screenLock() Check {
	c := Check{ID: "screen-lock", Name: "Password after sleep", Pane: paneLock}
	s := out("sysadminctl", "-screenLock", "status")
	switch {
	case strings.Contains(s, "immediate"):
		c.Status, c.Summary = Good, "Required immediately"
	case strings.Contains(s, "screenLock is off"):
		c.Status, c.Summary = Risk, "Not required"
		c.Advice = "Anyone can wake the Mac and use it. Require a password in Lock Screen settings."
	case lockDelay.MatchString(s):
		secs, _ := strconv.Atoi(lockDelay.FindStringSubmatch(s)[1])
		c.Summary = fmt.Sprintf("Required after %s", humanSecs(secs))
		c.Status = Good
		if secs > 60 {
			c.Status = Notice
			c.Advice = "Someone could use the Mac in that window. \"Immediately\" is safest."
		}
	default:
		c.Status, c.Summary = Unknown, "Couldn't read"
	}
	return c
}

func humanSecs(s int) string {
	if s >= 3600 {
		return fmt.Sprintf("%d hours", s/3600)
	}
	if s >= 60 {
		return fmt.Sprintf("%d minutes", s/60)
	}
	return fmt.Sprintf("%d seconds", s)
}

// sharing looks for listening services instead of reading settings (no admin needed).
func sharing() Check {
	c := Check{ID: "sharing", Name: "Sharing services", Pane: paneSharing}
	s, _ := sh.Quiet("netstat", "-anp", "tcp")
	ports := map[string]string{"22": "Remote Login (SSH)", "5900": "Screen Sharing", "3283": "Remote Management", "445": "File Sharing"}
	var on []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.Contains(line, "LISTEN") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		local := f[3]
		port := local[strings.LastIndexAny(local, ".:")+1:]
		if name, ok := ports[port]; ok && !contains(on, name) {
			on = append(on, name)
		}
	}
	if len(on) == 0 {
		c.Status, c.Summary = Good, "None listening"
		return c
	}
	c.Status, c.Summary = Notice, strings.Join(on, ", ")+" on"
	c.Advice = "Others on your network can connect to these. Turn off the ones you don't use."
	return c
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func airDrop() Check {
	c := Check{ID: "airdrop", Name: "AirDrop", Pane: paneAirDrop}
	v, ok := defaultsRead("com.apple.sharingd", "DiscoverableMode")
	switch {
	case !ok:
		c.Status, c.Summary = Good, "Contacts only (default)"
	case v == "Everyone":
		c.Status, c.Summary = Notice, "Everyone"
		c.Advice = "Strangers nearby can send you files. \"Contacts Only\" is enough most of the time."
	case v == "Off":
		c.Status, c.Summary = Good, "Off"
	default:
		c.Status, c.Summary = Good, v
	}
	return c
}

func findMy() Check {
	c := Check{ID: "find-my", Name: "Find My Mac", Pane: paneAppleID}
	if _, err := sh.Quiet("nvram", "fmm-mobileme-token-FMM"); err == nil {
		c.Status, c.Summary = Good, "On"
	} else {
		c.Status, c.Summary = Notice, "Off"
		c.Advice = "Lets you locate, lock or erase the Mac if it's lost. Turn it on in Apple Account → iCloud."
	}
	return c
}

func autoLogin() Check {
	c := Check{ID: "auto-login", Name: "Automatic login", Pane: paneUsers}
	if u, ok := defaultsRead("/Library/Preferences/com.apple.loginwindow", "autoLoginUser"); ok && u != "" {
		c.Status, c.Summary = Risk, "On for "+u
		c.Advice = "Anyone who restarts the Mac gets straight in, and FileVault can't protect it. Turn it off in Users & Groups."
	} else {
		c.Status, c.Summary = Good, "Off"
	}
	return c
}

func guest() Check {
	c := Check{ID: "guest", Name: "Guest account", Pane: paneUsers}
	if v, ok := defaultsRead("/Library/Preferences/com.apple.loginwindow", "GuestEnabled"); ok && v == "1" {
		c.Status, c.Summary = Notice, "On"
		c.Advice = "Anyone can log in without a password (with limited access)."
	} else {
		c.Status, c.Summary = Good, "Off"
	}
	return c
}

// appPermissions counts apps with powerful permissions. macOS keeps these in TCC databases
// that are only readable with Full Disk Access.
func appPermissions() Check {
	c := Check{ID: "app-permissions", Name: "Apps with powerful access", Pane: panePrivacy + "?Privacy_AllFiles"}
	if !fsx.HasFullDiskAccess() {
		c.Status, c.Summary = Unknown, "Needs Full Disk Access to read"
		c.Advice = "Review them in Privacy & Security → Full Disk Access, Screen Recording and Accessibility."
		return c
	}
	services := map[string]string{"kTCCServiceSystemPolicyAllFiles": "Full Disk Access", "kTCCServiceScreenCapture": "Screen Recording",
		"kTCCServiceAccessibility": "Accessibility", "kTCCServiceCamera": "Camera", "kTCCServiceMicrophone": "Microphone"}
	counts := map[string]int{}
	for _, db := range []string{"/Library/Application Support/com.apple.TCC/TCC.db", filepath.Join(fsx.H("Library/Application Support/com.apple.TCC"), "TCC.db")} {
		rows, err := sh.Quiet("sqlite3", "-readonly", db, "SELECT service, COUNT(*) FROM access WHERE auth_value = 2 GROUP BY service;")
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(rows), "\n") {
			if svc, n, ok := strings.Cut(line, "|"); ok {
				if label, ok := services[svc]; ok {
					k, _ := strconv.Atoi(n)
					counts[label] += k
				}
			}
		}
	}
	var parts []string
	for _, label := range []string{"Full Disk Access", "Screen Recording", "Accessibility", "Camera", "Microphone"} {
		if counts[label] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", label, counts[label]))
		}
	}
	if len(parts) == 0 {
		c.Status, c.Summary = Good, "None granted"
		return c
	}
	c.Status, c.Summary = Notice, strings.Join(parts, " · ")
	c.Advice = "Remove apps you no longer use from these lists in Privacy & Security."
	return c
}
