// Package sh runs external commands.
package sh

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Has reports whether a binary is on PATH.
func Has(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// Quiet runs a command with a timeout and returns its stdout.
func Quiet(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
	return string(out), err
}

// Line runs a command and returns the last line of its output, trimmed.
func Line(args ...string) string {
	out, err := Quiet(args...)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Shown runs a command with its output going to the terminal.
func Shown(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
