package doctor

import (
	"testing"
	"time"
)

func TestParseLoadAvg(t *testing.T) {
	one, five, fifteen, err := parseLoadAvg("{ 26.54 20.43 18.01 }")
	if err != nil || one != 26.54 || five != 20.43 || fifteen != 18.01 {
		t.Fatalf("got %v %v %v %v", one, five, fifteen, err)
	}
	if _, _, _, err := parseLoadAvg("garbage"); err == nil {
		t.Error("want error for bad input")
	}
}

func TestParseSwap(t *testing.T) {
	total, used, err := parseSwap("total = 13312.00M  used = 12288.19M  free = 1023.81M  (encrypted)")
	if err != nil || total != 13312<<20 || used < 12288<<20 || used > 12289<<20 {
		t.Fatalf("got %d %d %v", total, used, err)
	}
}

func TestParseEtime(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"02:35":       2*time.Minute + 35*time.Second,
		"10:00:30":    10*time.Hour + 30*time.Second,
		"02-16:58:42": 2*24*time.Hour + 16*time.Hour + 58*time.Minute + 42*time.Second,
	} {
		if got := parseEtime(in); got != want {
			t.Errorf("parseEtime(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParsePS(t *testing.T) {
	out := `  612 140.2 204800 02-16:58:42 /opt/homebrew/opt/mysql@9.7/bin/mysqld
  900  40.8 614400 03-19:56:42 /Applications/Claude.app/Contents/Frameworks/Claude Helper (Renderer).app/Contents/MacOS/Claude Helper (Renderer)
  901   1.0 102400       02:00 /Applications/Claude.app/Contents/MacOS/Claude`
	ps := parsePS(out)
	if len(ps) != 3 {
		t.Fatalf("want 3 processes, got %d", len(ps))
	}
	if ps[0].PID != 612 || ps[0].CPU != 140.2 || ps[0].RSS != 204800*1024 || ps[0].Name() != "mysqld" {
		t.Errorf("bad first process %+v", ps[0])
	}
	if ps[1].Name() != "Claude" || ps[1].Path == "" {
		t.Errorf("helper should group under its app, got %q from %q", ps[1].Name(), ps[1].Path)
	}
	apps := groupByApp(ps)
	if len(apps) != 2 || apps[1].Count != 2 {
		t.Errorf("want mysqld + Claude(2), got %+v", apps)
	}
}

func TestBrewServiceFor(t *testing.T) {
	svcs := []string{"mysql@9.7", "redis"}
	if got := brewServiceFor("/opt/homebrew/opt/mysql@9.7/bin/mysqld", svcs); got != "mysql@9.7" {
		t.Errorf("got %q", got)
	}
	if got := brewServiceFor("/opt/homebrew/Cellar/redis/7.2/bin/redis-server", svcs); got != "redis" {
		t.Errorf("got %q", got)
	}
	if got := brewServiceFor("/usr/sbin/mysqld", svcs); got != "" {
		t.Errorf("non-Homebrew binary matched %q", got)
	}
}
