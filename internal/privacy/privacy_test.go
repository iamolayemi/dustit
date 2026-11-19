package privacy

import "testing"

func TestRunReturnsKnownChecks(t *testing.T) {
	checks := Run()
	if len(checks) < 10 {
		t.Fatalf("expected at least 10 checks, got %d", len(checks))
	}
	seen := map[string]bool{}
	for _, c := range checks {
		if c.ID == "" || c.Name == "" || c.Pane == "" || c.Summary == "" {
			t.Errorf("incomplete check %+v", c)
		}
		if seen[c.ID] {
			t.Errorf("duplicate id %s", c.ID)
		}
		seen[c.ID] = true
		switch c.Status {
		case Good, Notice, Risk, Unknown:
		default:
			t.Errorf("%s: bad status %q", c.ID, c.Status)
		}
	}
}

func TestHumanSecs(t *testing.T) {
	for in, want := range map[int]string{5: "5 seconds", 300: "5 minutes", 7200: "2 hours"} {
		if got := humanSecs(in); got != want {
			t.Errorf("humanSecs(%d) = %q, want %q", in, got, want)
		}
	}
}
