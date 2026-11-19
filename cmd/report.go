package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iamolayemi/dustit/internal/doctor"
	"github.com/iamolayemi/dustit/internal/finder"
	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/report"
	"github.com/iamolayemi/dustit/internal/targets"
	"github.com/iamolayemi/dustit/internal/timeline"
	"github.com/iamolayemi/dustit/internal/ui"
)

func newReportCmd() *cobra.Command {
	var out string
	var open, noDoctor, noMap bool
	c := &cobra.Command{
		Use:   "report",
		Short: "Write an HTML report: disk treemap, cleanup categories and performance check",
		Long: `Writes a single, self-contained HTML page (no external files or tracking) with a treemap
of your home folder, every cleanup category and the doctor's findings. Nothing is deleted.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			stop := ui.Spin("Scanning… (read-only; the treemap takes a minute on a full disk)")
			var (
				results  []targets.Result
				findings []doctor.Finding
				tree     *finder.Node
			)
			done := make(chan struct{}, 3)
			go func() { results = targets.ScanAll(targets.All()); done <- struct{}{} }()
			go func() {
				if !noDoctor {
					findings = doctor.Run(ui.Human)
				}
				done <- struct{}{}
			}()
			go func() {
				if !noMap {
					tree = finder.Tree(fsx.Home, 100000)
				}
				done <- struct{}{}
			}()
			for i := 0; i < 3; i++ {
				<-done
			}
			stop()

			free, total := fsx.DiskTotals()
			safe, review, _ := targets.Totals(results)
			host, _ := os.Hostname()
			d := report.Data{Generated: time.Now(), Version: version, Host: strings.TrimSuffix(host, ".local"), Free: free, Total: total,
				Safe: safe, Review: review, Human: ui.Human}
			if tree != nil {
				timeline.SaveIfStale(timeline.FromTree(tree), 6*time.Hour)
				d.Groups = finder.StorageFrom(tree)
				d.Tree = tree.Folded(12)
			}
			sort.SliceStable(results, func(i, j int) bool { return results[i].Bytes > results[j].Bytes })
			for _, r := range results {
				if r.Unavailable != "" {
					continue
				}
				d.Categories = append(d.Categories, report.Category{ID: r.T.ID, Name: r.T.Name, Desc: r.T.Desc,
					Risk: r.T.Risk.String(), Bytes: r.Bytes, Items: len(r.Items), Known: r.SizeKnown()})
			}
			for _, f := range findings {
				short := f.Short
				if short == "" {
					short = f.Summary
				}
				d.Findings = append(d.Findings, report.Finding{Check: f.Check, Severity: severityNames[f.Severity], Summary: f.Summary, Short: short, Advice: f.Advice})
			}
			html, err := report.Render(d)
			if err != nil {
				return err
			}
			if out == "" {
				out = "dustit-report-" + time.Now().Format("2006-01-02") + ".html"
			}
			abs, _ := filepath.Abs(out)
			if err := os.WriteFile(abs, html, 0o644); err != nil {
				return err
			}
			fmt.Printf("%s Report written to %s\n", ui.Green.Render("✓"), fsx.Pretty(abs))
			if open {
				_ = exec.Command("open", abs).Run()
			} else {
				ui.Hint("Open it with: open %q   (or pass --open)", fsx.Pretty(abs))
			}
			return nil
		},
	}
	c.Flags().StringVarP(&out, "output", "o", "", "file to write (default: dustit-report-DATE.html here)")
	c.Flags().BoolVar(&open, "open", false, "open the report in your browser")
	c.Flags().BoolVar(&noDoctor, "no-doctor", false, "skip the performance check")
	c.Flags().BoolVar(&noMap, "no-treemap", false, "skip the treemap (faster)")
	return c
}
