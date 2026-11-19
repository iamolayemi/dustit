package schedule

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// AlertLabel is the launchd label of the low-disk alert.
const AlertLabel = Label + ".alert"

// AlertPlistPath is where the alert agent lives.
func AlertPlistPath() string {
	return filepath.Join(home(), "Library", "LaunchAgents", AlertLabel+".plist")
}

// AlertSpec says how often to check and the threshold ("10%" or "20GB").
type AlertSpec struct {
	Binary string
	Below  string
	Every  time.Duration
}

var alertTmpl = template.Must(template.New("alert").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Binary}}</string>
		<string>alert</string>
		<string>--below</string>
		<string>{{.Below}}</string>
		<string>--notify</string>
	</array>
	<key>StartInterval</key>
	<integer>{{.Seconds}}</integer>
	<key>RunAtLoad</key>
	<true/>
	<key>LowPriorityIO</key>
	<true/>
</dict>
</plist>
`))

// AlertPlist renders the alert agent.
func AlertPlist(s AlertSpec) (string, error) {
	if s.Binary == "" || s.Below == "" || s.Every < time.Minute {
		return "", fmt.Errorf("alert needs a binary, a threshold and an interval of at least a minute")
	}
	var b bytes.Buffer
	err := alertTmpl.Execute(&b, map[string]any{"Label": AlertLabel, "Binary": xmlEscape(s.Binary),
		"Below": xmlEscape(s.Below), "Seconds": int(s.Every.Seconds())})
	return b.String(), err
}

// InstallAlert writes and loads the alert agent.
func InstallAlert(s AlertSpec) error {
	content, err := AlertPlist(s)
	if err != nil {
		return err
	}
	return install(AlertPlistPath(), content)
}

// RemoveAlert unloads and deletes the alert agent.
func RemoveAlert() error { return uninstall(AlertPlistPath()) }

// AlertInstalled returns the installed alert's threshold and interval, if any.
func AlertInstalled() (below string, every time.Duration, ok bool) {
	out, err := exec.Command("plutil", "-extract", "ProgramArguments.3", "raw", "-o", "-", AlertPlistPath()).Output()
	if err != nil {
		return "", 0, false
	}
	secs, _ := exec.Command("plutil", "-extract", "StartInterval", "raw", "-o", "-", AlertPlistPath()).Output()
	n, _ := strconv.Atoi(strings.TrimSpace(string(secs)))
	return strings.TrimSpace(string(out)), time.Duration(n) * time.Second, true
}

func install(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "bootout", domain(), path).Run() // ignore "not loaded"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	if out, err := exec.Command("launchctl", "bootstrap", domain(), path).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func uninstall(path string) error {
	_ = exec.Command("launchctl", "bootout", domain(), path).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Threshold parses "10%" or "20GB" into a check on (free, total).
func Threshold(s string, parseSize func(string) (int64, error)) (func(free, total int64) bool, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		pct, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil || pct <= 0 || pct >= 100 {
			return nil, fmt.Errorf("bad percentage %q", s)
		}
		return func(free, total int64) bool { return float64(free) < float64(total)*pct/100 }, nil
	}
	n, err := parseSize(s)
	if err != nil || n <= 0 {
		return nil, fmt.Errorf("bad threshold %q (use 10%% or 20GB)", s)
	}
	return func(free, _ int64) bool { return free < n }, nil
}

// ShouldNotify throttles alerts to one per `gap`, remembered in a small state file.
func ShouldNotify(gap time.Duration) bool {
	state := filepath.Join(filepath.Dir(LogPath()), "alert-last")
	if fi, err := os.Stat(state); err == nil && time.Since(fi.ModTime()) < gap {
		return false
	}
	_ = os.MkdirAll(filepath.Dir(state), 0o755)
	_ = os.WriteFile(state, []byte(time.Now().Format(time.RFC3339)), 0o644)
	return true
}

// WatchLabel is the launchd label of the background watcher.
const WatchLabel = Label + ".watch"

// WatchPlistPath is where the watcher agent lives.
func WatchPlistPath() string {
	return filepath.Join(home(), "Library", "LaunchAgents", WatchLabel+".plist")
}

var intervalTmpl = template.Must(template.New("interval").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
{{- range .Args}}
		<string>{{.}}</string>
{{- end}}
	</array>
	<key>StartInterval</key>
	<integer>{{.Seconds}}</integer>
	<key>RunAtLoad</key>
	<true/>
	<key>LowPriorityIO</key>
	<true/>
	<key>Nice</key>
	<integer>15</integer>
	<key>StandardErrorPath</key>
	<string>{{.Log}}</string>
</dict>
</plist>
`))

// WatchPlist renders the watcher agent, which runs "dustit watch --notify" every interval.
func WatchPlist(binary string, every time.Duration) (string, error) {
	if binary == "" || every < time.Minute {
		return "", fmt.Errorf("watch needs a binary and an interval of at least a minute")
	}
	var args []string
	for _, a := range []string{binary, "watch", "--notify", "--no-color"} {
		args = append(args, xmlEscape(a))
	}
	var b bytes.Buffer
	err := intervalTmpl.Execute(&b, map[string]any{"Label": WatchLabel, "Args": args, "Seconds": int(every.Seconds()), "Log": xmlEscape(LogPath())})
	return b.String(), err
}

// InstallWatch writes and loads the watcher agent.
func InstallWatch(binary string, every time.Duration) error {
	content, err := WatchPlist(binary, every)
	if err != nil {
		return err
	}
	return install(WatchPlistPath(), content)
}

// RemoveWatch unloads and deletes the watcher agent.
func RemoveWatch() error { return uninstall(WatchPlistPath()) }

// WatchInstalled reports whether the watcher agent is installed, and its interval.
func WatchInstalled() (time.Duration, bool) {
	secs, err := exec.Command("plutil", "-extract", "StartInterval", "raw", "-o", "-", WatchPlistPath()).Output()
	if err != nil {
		return 0, false
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(secs)))
	return time.Duration(n) * time.Second, true
}
