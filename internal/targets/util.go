package targets

import (
	"strconv"
	"strings"
)

func joinArgs(a []string) string { return strings.Join(a, " ") }
func itoa(n int) string          { return strconv.Itoa(n) }
