package targets

import (
	"strconv"
	"strings"
)

func joinArgs(a []string) string { return strings.Join(a, " ") }
func itoa(n int) string          { return strconv.Itoa(n) }

// displayCmd shows a command readably; `sh -c script sh arg` is shown as the script with the arg filled in.
func displayCmd(a []string) string {
	if len(a) >= 5 && a[0] == "sh" && a[1] == "-c" {
		return strings.ReplaceAll(a[2], `"$1"`, `"`+a[4]+`"`)
	}
	return joinArgs(a)
}
