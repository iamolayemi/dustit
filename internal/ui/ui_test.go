package ui

import "testing"

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"500MB": 5e8, "1G": 1e9, "1.5gb": 1.5e9, "0": 0, "200": 200} {
		if got, err := ParseSize(in); err != nil || got != want {
			t.Errorf("ParseSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
}

func TestHuman(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 999: "999 B", 1500: "1.5 KB", 16_800_000_000: "16.8 GB", -5: "0 B"} {
		if got := Human(in); got != want {
			t.Errorf("Human(%d) = %q, want %q", in, got, want)
		}
	}
}
