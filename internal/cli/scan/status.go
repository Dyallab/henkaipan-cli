package scan

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dyallab/henkaipan-cli/internal/api"
	"github.com/dyallab/henkaipan-cli/internal/config"
	"github.com/dyallab/henkaipan-cli/internal/output"
	"github.com/dyallab/henkaipan-cli/internal/severity"
)

func newStatusCmd() *cobra.Command {
	var (
		wait         bool
		pollInterval int
		failOn       string
	)

	cmd := &cobra.Command{
		Use:   "status <scan-id>",
		Short: "Query scan status (or wait for completion)",
		Args:  cobra.ExactArgs(1),
		Long: `Read /api/v1/scans/{id}/status once and print it.

With --wait, the command polls until the scan reaches a terminal state
(completed or failed), then exits non-zero if --fail-on severity is met.`,
		Example: `  henkaipan scan status <scan-id>
  henkaipan scan status <scan-id> --wait --fail-on high`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			cfg, err := config.Load(viper.GetViper())
			if err != nil {
				return err
			}
			format, err := output.ParseFormat(cfg.Output)
			if err != nil {
				return err
			}
			threshold, err := severity.Parse(failOn)
			if err != nil {
				return err
			}

			client := api.NewClient(cfg)
			ctx, cancel := context.WithTimeout(context.Background(),
				time.Duration(cfg.TimeoutSeconds)*time.Second)
			defer cancel()

			if !wait {
				st, err := client.ScanStatus(ctx, id)
				if err != nil {
					return err
				}
				return renderStatus(format, st)
			}

			return pollUntilDone(ctx, client, id, pollInterval, threshold, format)
		},
	}

	cmd.Flags().BoolVar(&wait, "wait", false, "Block until the scan completes")
	cmd.Flags().IntVar(&pollInterval, "poll-interval", 15, "Seconds between polls")
	cmd.Flags().StringVar(&failOn, "fail-on", "", "Exit code 1 if any finding meets or exceeds this severity")

	return cmd
}

func renderStatus(format output.Format, st *api.ScanStatusResponse) error {
	if format == output.JSON {
		return output.WriteJSON(os.Stdout, st)
	}
	if format == output.YAML {
		return output.WriteYAML(os.Stdout, st)
	}
	output.Tableize(os.Stdout,
		[]string{"ID", "STATUS", "BRANCH", "PROJECT", "STARTED", "FINISHED"},
		[][]string{{
			st.Scan.ID,
			st.Scan.Status,
			st.Scan.Branch,
			st.Scan.ProjectID,
			st.Scan.StartedAt,
			st.Scan.FinishedAt,
		}})
	if len(st.Findings) > 0 {
		counts := severity.FromStrings(findSeverities(st.Findings))
		fmt.Fprintf(os.Stdout, "\nFindings: critical=%d high=%d medium=%d low=%d\n",
			counts.Critical, counts.High, counts.Medium, counts.Low)
	}
	return nil
}

func findSeverities(fs []api.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Severity)
	}
	return out
}

func pollUntilDone(ctx context.Context, client *api.Client, id string,
	pollInterval int, threshold severity.Level, format output.Format) error {
	if pollInterval < 1 {
		pollInterval = 15
	}
	totals := severity.Counts{}
	timeout := 20 * time.Minute
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
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
			if err := renderStatus(format, st); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "\nResults: critical=%d high=%d medium=%d low=%d\n",
				totals.Critical, totals.High, totals.Medium, totals.Low)
			if totals.ExceedsThreshold(threshold) {
				return fmt.Errorf("findings meet or exceed --fail-on=%s threshold", threshold)
			}
			return nil
		case "failed":
			return fmt.Errorf("scan %s failed", id)
		}
		select {
		case <-pollCtx.Done():
			return fmt.Errorf("timed out after %s waiting for scan %s", timeout, id)
		case <-time.After(time.Duration(pollInterval) * time.Second):
		}
	}
}