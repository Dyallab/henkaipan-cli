package findings

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

func newListCmd() *cobra.Command {
	var (
		severityFilter string
		statusFilter   string
		scannerFilter  string
		projectID      string
		page           int
		pageSize       int
		failOn         string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List findings with optional filters",
		Long: `Query /api/findings with optional filters and pagination.

Severity, status, and scanner are exact-match filters. Combine them with
--project-id to scope a query, and use --page / --page-size to paginate.

If --fail-on is set, the process exits 1 when any finding meets or exceeds
that severity (useful for ad-hoc CI gates).`,
		Example: `  henkaipan findings list --severity high --project-id <uuid>
  henkaipan findings list --scanner semgrep --page 2 --page-size 50
  henkaipan findings list --fail-on critical`,
		RunE: func(cmd *cobra.Command, _ []string) error {
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

			resp, err := client.ListFindings(ctx, api.FindingsFilter{
				Severity:  severityFilter,
				Status:    statusFilter,
				Scanner:   scannerFilter,
				ProjectID: projectID,
				Page:      page,
				PageSize:  pageSize,
			})
			if err != nil {
				return err
			}

			counts := severity.FromStrings(extractSeverities(resp.Findings))
			renderFindings(format, resp)

			fmt.Fprintf(os.Stdout, "\nPage %d/%d — total: %d (critical=%d high=%d medium=%d low=%d)\n",
				resp.Page, totalPages(resp), resp.Total,
				counts.Critical, counts.High, counts.Medium, counts.Low)

			if counts.ExceedsThreshold(threshold) {
				return fmt.Errorf("findings meet or exceed --fail-on=%s threshold", threshold)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&severityFilter, "severity", "", "Filter by severity (critical|high|medium|low)")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by finding status (e.g. open, fixed)")
	cmd.Flags().StringVar(&scannerFilter, "scanner", "", "Filter by scanner name (semgrep, trivy, gitleaks, grype, nuclei)")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Scope to a project UUID")
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-indexed)")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Results per page")
	cmd.Flags().StringVar(&failOn, "fail-on", "", "Exit code 1 if any finding meets or exceeds this severity")

	return cmd
}

func extractSeverities(fs []api.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Severity)
	}
	return out
}

func totalPages(resp *api.FindingsListResponse) int {
	if resp.PageSize <= 0 {
		return 1
	}
	pages := resp.Total / resp.PageSize
	if resp.Total%resp.PageSize != 0 {
		pages++
	}
	if pages == 0 {
		pages = 1
	}
	return pages
}

func renderFindings(format output.Format, resp *api.FindingsListResponse) {
	if format == output.JSON {
		_ = output.WriteJSON(os.Stdout, resp)
		return
	}
	if format == output.YAML {
		_ = output.WriteYAML(os.Stdout, resp)
		return
	}
	rows := make([][]string, 0, len(resp.Findings))
	for _, f := range resp.Findings {
		rows = append(rows, []string{
			f.ID, f.Scanner, f.Severity, f.Status, f.Title, f.File,
		})
	}
	output.Tableize(os.Stdout,
		[]string{"ID", "SCANNER", "SEVERITY", "STATUS", "TITLE", "FILE"},
		rows)
}