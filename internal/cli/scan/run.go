package scan

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dyallab/henkaipan-cli/internal/api"
	"github.com/dyallab/henkaipan-cli/internal/config"
	"github.com/dyallab/henkaipan-cli/internal/output"
	"github.com/dyallab/henkaipan-cli/internal/severity"
)

func newRunCmd() *cobra.Command {
	var (
		repoURL      string
		projectID    string
		scanners     string
		branch       string
		autoCreate   bool
		wait         bool
		failOn       string
		pollInterval int
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Trigger a scan against the HenKaiPan API",
		Long: `Trigger a scan via POST /api/v1/scans/external.

Exactly one of --project-id or --repo-url must be supplied. When --project-id
is empty and --auto-create-project is true (default), the project is created
from the repo URL on first scan — mirroring henkaipan-action's behavior.

If --wait is set, this command blocks until the scan completes (or fails),
then exits non-zero if --fail-on severity is met.`,
		Example: `  # Auto-create project from repo URL, default scanners
  henkaipan scan run --repo-url https://github.com/owner/repo

  # Specific project + scanners + branch, wait and fail on high findings
  henkaipan scan run --project-id <uuid> --scanners semgrep,trivy --branch main \
      --wait --fail-on high`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(viper.GetViper())
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			if repoURL == "" && projectID == "" {
				return fmt.Errorf("either --repo-url or --project-id is required")
			}
			if repoURL != "" && projectID != "" {
				return fmt.Errorf("--repo-url and --project-id are mutually exclusive")
			}
			if !autoCreate && projectID == "" {
				return fmt.Errorf("--project-id is required when --auto-create-project is false")
			}

			threshold, err := severity.Parse(failOn)
			if err != nil {
				return err
			}

			scannerList := splitAndTrim(scanners)
			if len(scannerList) == 0 {
				scannerList = []string{"all"}
			}

			client := api.NewClient(cfg)
			ctx, cancel := context.WithTimeout(context.Background(),
				time.Duration(cfg.TimeoutSeconds)*time.Second)
			defer cancel()

			resp, err := client.TriggerScan(ctx, api.ScanRunRequest{
				RepoURL:   repoURL,
				Scanners:  scannerList,
				Branch:    branch,
				ProjectID: projectID,
			})
			if err != nil {
				return err
			}

			// Default to a short human summary so users see what happened.
			fmt.Fprintf(os.Stdout, "Scan accepted: batch=%s ids=%s\n",
				resp.BatchID, strings.Join(resp.ScanIDs, ","))

			if !wait {
				return renderRunOutput(format, resp)
			}

			return waitForScans(ctx, client, resp.ScanIDs, pollInterval, threshold, format)
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo-url", "",
		"Repository URL to scan (auto-creates the project on first run)")
	cmd.Flags().StringVar(&projectID, "project-id", "",
		"UUID of an existing HenKaiPan project to scan")
	cmd.Flags().StringVar(&scanners, "scanners", "all",
		"Comma-separated scanners or pack (all, sast, sca, secrets, vuln, containers)")
	cmd.Flags().StringVar(&branch, "branch", "",
		"Git branch to scan (defaults to the project's default branch)")
	cmd.Flags().BoolVar(&autoCreate, "auto-create-project", true,
		"When --project-id is empty, auto-create the project from --repo-url")
	cmd.Flags().BoolVar(&wait, "wait", false,
		"Block until the triggered scans complete")
	cmd.Flags().StringVar(&failOn, "fail-on", "",
		"Exit code 1 if any finding meets or exceeds this severity (critical, high, medium, low, none)")
	cmd.Flags().IntVar(&pollInterval, "poll-interval", 15,
		"Seconds between status polls when --wait is set")

	return cmd
}

func renderRunOutput(format output.Format, resp *api.ScanRunResponse) error {
	if format == output.JSON {
		return output.WriteJSON(os.Stdout, resp)
	}
	if format == output.YAML {
		return output.WriteYAML(os.Stdout, resp)
	}
	// table view of just the IDs is fine; users get the richer output from --wait
	rows := [][]string{{resp.BatchID, strings.Join(resp.ScanIDs, ",")}}
	output.Tableize(os.Stdout, []string{"BATCH ID", "SCAN IDS"}, rows)
	return nil
}

func waitForScans(ctx context.Context, client *api.Client, ids []string,
	pollInterval int, threshold severity.Level, format output.Format) error {
	if pollInterval < 1 {
		pollInterval = 15
	}
	totals := severity.Counts{}
	anyFailed := false
	timeout := 20 * time.Minute
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		allDone := true
		for _, id := range ids {
			st, err := client.ScanStatus(pollCtx, id)
			if err != nil {
				return err
			}
			switch st.Scan.Status {
			case "completed":
				for _, f := range st.Findings {
					if l, err := severity.Parse(f.Severity); err == nil {
						switch l {
						case severity.Critical:
							totals.Critical++
						case severity.High:
							totals.High++
						case severity.Medium:
							totals.Medium++
						case severity.Low:
							totals.Low++
						}
					}
				}
				fmt.Fprintf(os.Stdout, "scan %s: completed\n", id)
			case "failed":
				anyFailed = true
				fmt.Fprintf(os.Stdout, "scan %s: failed\n", id)
			case "running", "pending":
				allDone = false
				fmt.Fprintf(os.Stdout, "scan %s: %s\n", id, st.Scan.Status)
			default:
				allDone = false
				fmt.Fprintf(os.Stdout, "scan %s: %s\n", id, st.Scan.Status)
			}
		}
		if allDone {
			break
		}
		select {
		case <-pollCtx.Done():
			return fmt.Errorf("timed out after %s waiting for scans", timeout)
		case <-time.After(time.Duration(pollInterval) * time.Second):
		}
	}

	// Render summary
	fmt.Fprintf(os.Stdout, "\nResults: critical=%d high=%d medium=%d low=%d (total=%d)\n",
		totals.Critical, totals.High, totals.Medium, totals.Low, totals.Total())
	if format == output.JSON {
		_ = output.WriteJSON(os.Stdout, totals)
	} else if format == output.YAML {
		_ = output.WriteYAML(os.Stdout, totals)
	}

	if anyFailed {
		return fmt.Errorf("one or more scans failed")
	}
	if totals.ExceedsThreshold(threshold) {
		return fmt.Errorf("findings meet or exceed --fail-on=%s threshold", threshold)
	}
	return nil
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}