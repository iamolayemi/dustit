// Package schedule installs a launchd agent that runs `macclean clean` periodically.
package schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

// Label is the launchd job label.
const Label = "com.github.iamolayemi.macclean"

// DefaultCategories are cleaned on a schedule unless the user picks others: the safe
// categories that can't surprise anyone. Dependency folders, the Trash and caches of
// apps that may be open are left for interactive runs.
var DefaultCategories = []string{
	"user-caches", "user-logs", "dot-cache", "xcode-derived", "xcode-device-support",
	"simulator-caches", "ios-updates", "npm-cache", "yarn-cache", "pnpm-store", "bun-cache",
	"homebrew", "gradle-cache", "composer-cache", "cargo-cache",
}

// Spec describes when and what to clean.
type Spec struct {
	Binary     string
	Categories []string
	Weekday    int // 0 = Sunday … 6; -1 = every day
	Hour       int
	Minute     int
	LogFile    string
}

func home() string { h, _ := os.UserHomeDir(); return h }

// PlistPath is where the agent lives.
func PlistPath() string { return filepath.Join(home(), "Library", "LaunchAgents", Label+".plist") }

// LogPath is where scheduled runs write their output (kept out of ~/Library/Logs,
// which macclean itself cleans).
func LogPath() string {
	return filepath.Join(home(), "Library", "Application Support", "macclean", "scheduled.log")
}

// Weekdays maps names to launchd weekday numbers.
var Weekdays = map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}

var plistTmpl = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Binary}}</string>
		<string>clean</string>
		<string>--yes</string>
		<string>--notify</string>
		<string>--no-color</string>
		<string>--only</string>
		<string>{{.Only}}</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
{{- if ge .Weekday 0}}
		<key>Weekday</key>
		<integer>{{.Weekday}}</integer>
{{- end}}
		<key>Hour</key>
		<integer>{{.Hour}}</integer>
		<key>Minute</key>
		<integer>{{.Minute}}</integer>
	</dict>
	<key>StandardOutPath</key>
	<string>{{.LogFile}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogFile}}</string>
	<key>LowPriorityIO</key>
	<true/>
	<key>Nice</key>
	<integer>10</integer>
</dict>
</plist>
`))

// Plist renders the launchd agent.
func Plist(s Spec) (string, error) {
	if s.Binary == "" || len(s.Categories) == 0 {
		return "", fmt.Errorf("schedule needs a binary and at least one category")
	}
	var b bytes.Buffer
	err := plistTmpl.Execute(&b, map[string]any{
		"Label": Label, "Binary": xmlEscape(s.Binary), "Only": xmlEscape(strings.Join(s.Categories, ",")),
		"Weekday": s.Weekday, "Hour": s.Hour, "Minute": s.Minute, "LogFile": xmlEscape(s.LogFile),
	})
	return b.String(), err
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// Binary picks a stable path to macclean: the one on PATH (e.g. /opt/homebrew/bin/macclean,
// which survives upgrades), else the running executable.
func Binary() (string, error) {
	if p, err := exec.LookPath("macclean"); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs, nil
		}
	}
	return os.Executable()
}

func domain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

// Install writes the agent and (re)loads it.
func Install(s Spec) error {
	content, err := Plist(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(PlistPath()), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.LogFile), 0o755); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "bootout", domain(), PlistPath()).Run() // ignore "not loaded"
	if err := os.WriteFile(PlistPath(), []byte(content), 0o644); err != nil {
		return err
	}
	if out, err := exec.Command("launchctl", "bootstrap", domain(), PlistPath()).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Remove unloads and deletes the agent.
func Remove() error {
	_ = exec.Command("launchctl", "bootout", domain(), PlistPath()).Run()
	if err := os.Remove(PlistPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Status describes the installed agent.
type Status struct {
	Installed  bool
	Loaded     bool
	Categories []string
	Weekday    int
	Hour       int
	Minute     int
}

// Current reads the installed agent, if any.
func Current() Status {
	var st Status
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", PlistPath()).Output()
	if err != nil {
		return st
	}
	var p struct {
		ProgramArguments      []string
		StartCalendarInterval struct {
			Weekday *int
			Hour    int
			Minute  int
		}
	}
	if json.Unmarshal(out, &p) != nil {
		return st
	}
	st.Installed = true
	st.Weekday = -1
	if p.StartCalendarInterval.Weekday != nil {
		st.Weekday = *p.StartCalendarInterval.Weekday
	}
	st.Hour, st.Minute = p.StartCalendarInterval.Hour, p.StartCalendarInterval.Minute
	for i, a := range p.ProgramArguments {
		if a == "--only" && i+1 < len(p.ProgramArguments) {
			st.Categories = strings.Split(p.ProgramArguments[i+1], ",")
		}
	}
	st.Loaded = exec.Command("launchctl", "print", domain()+"/"+Label).Run() == nil
	return st
}

// Describe is a human sentence for a schedule ("every Monday at 10:00").
func Describe(weekday, hour, minute int) string {
	when := "every day"
	if weekday >= 0 {
		names := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
		when = "every " + names[weekday%7]
	}
	return fmt.Sprintf("%s at %02d:%02d", when, hour, minute)
}
