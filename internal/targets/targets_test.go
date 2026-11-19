package targets

import "testing"

func TestIDsUniqueAndDescribed(t *testing.T) {
	seen := map[string]bool{}
	for _, tg := range All() {
		if seen[tg.ID] {
			t.Errorf("duplicate id %q", tg.ID)
		}
		seen[tg.ID] = true
		if tg.Name == "" || tg.Desc == "" {
			t.Errorf("%q needs a name and description", tg.ID)
		}
		if len(tg.Dirs) == 0 && tg.Find == nil && len(tg.CleanCmd) == 0 {
			t.Errorf("%q has nothing to clean", tg.ID)
		}
	}
}

func TestSelect(t *testing.T) {
	if _, err := Select([]string{"nope"}, false); err == nil {
		t.Error("unknown ids must be rejected")
	}
	def, _ := Select(nil, false)
	for _, tg := range def {
		if tg.Risk != Safe {
			t.Errorf("default selection must be safe-only, got %q", tg.ID)
		}
	}
	all, _ := Select(nil, true)
	if len(all) != len(All()) {
		t.Error("--all must select every category")
	}
}
