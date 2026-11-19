package cmd

import (
	"encoding/json"
	"os"

	"github.com/iamolayemi/macclean/internal/doctor"
	"github.com/iamolayemi/macclean/internal/finder"
	"github.com/iamolayemi/macclean/internal/targets"
)

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type jsonItem struct {
	Path string `json:"path"`
	Size int64  `json:"size_bytes"`
	Note string `json:"note,omitempty"`
}

type jsonCategory struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Risk        string     `json:"risk"`
	Bytes       int64      `json:"size_bytes"`
	SizeKnown   bool       `json:"size_known"`
	UpTo        bool       `json:"size_is_upper_bound,omitempty"`
	ItemCount   int        `json:"item_count"`
	Unavailable string     `json:"unavailable,omitempty"`
	KeptRunning int        `json:"kept_because_app_is_open,omitempty"`
	Items       []jsonItem `json:"items,omitempty"`
}

type jsonScan struct {
	FreeBytes   int64            `json:"free_bytes"`
	SafeBytes   int64            `json:"safe_bytes"`
	ReviewBytes int64            `json:"review_bytes"`
	Unreadable  int              `json:"unreadable_paths"`
	Categories  []jsonCategory   `json:"categories"`
	Insights    []finder.Insight `json:"insights,omitempty"`
}

func scanJSON(results []targets.Result, insights []finder.Insight, free int64, withItems bool) jsonScan {
	safe, review, denied := targets.Totals(results)
	out := jsonScan{FreeBytes: free, SafeBytes: safe, ReviewBytes: review, Unreadable: denied, Insights: insights}
	for _, r := range results {
		c := jsonCategory{ID: r.T.ID, Name: r.T.Name, Description: r.T.Desc, Risk: r.T.Risk.String(), Bytes: r.Bytes,
			SizeKnown: r.SizeKnown(), UpTo: r.T.UpTo, ItemCount: len(r.Items), Unavailable: r.Unavailable, KeptRunning: r.KeptRunning}
		if withItems {
			for _, it := range r.Items {
				c.Items = append(c.Items, jsonItem{it.Path, it.Size, it.Note})
			}
		}
		out.Categories = append(out.Categories, c)
	}
	return out
}

var severityNames = map[doctor.Severity]string{doctor.OK: "ok", doctor.Info: "info", doctor.Warn: "warn", doctor.Bad: "bad"}

type jsonFinding struct {
	Check    string   `json:"check"`
	Severity string   `json:"severity"`
	Summary  string   `json:"summary"`
	Details  []string `json:"details,omitempty"`
	Advice   []string `json:"advice,omitempty"`
	Fixes    []string `json:"fixes,omitempty"`
}

func doctorJSON(fs []doctor.Finding) []jsonFinding {
	out := make([]jsonFinding, 0, len(fs))
	for _, f := range fs {
		j := jsonFinding{Check: f.Check, Severity: severityNames[f.Severity], Summary: f.Summary, Details: f.Details, Advice: f.Advice}
		for _, x := range f.Fixes {
			j.Fixes = append(j.Fixes, x.Description)
		}
		out = append(out, j)
	}
	return out
}
