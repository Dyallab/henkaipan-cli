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
)

func newExportCmd() *cobra.Command {
	var (
		format         string
		severityFilter string
		statusFilter   string
		scannerFilter  string
		projectID      string
		outputPath     string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export findings to JSON or CSV",
		Long: `Stream /api/findings/export to stdout (or to --output) in the requested format.

The format is independent of --output rendering: --format json writes raw JSON
findings, --format csv writes raw CSV. Use --output to redirect to a file.`,
		Example: `  henkaipan findings export --format json --severity critical
  henkaipan findings export --format csv --project-id <uuid> --output findings.csv`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(viper.GetViper())
			if err != nil {
				return err
			}

			switch format {
			case "json", "csv":
			default:
				return fmt.Errorf("--format must be json or csv")
			}

			client := api.NewClient(cfg)
			ctx, cancel := context.WithTimeout(context.Background(),
				time.Duration(cfg.TimeoutSeconds)*time.Second)
			defer cancel()

			body, err := client.ExportFindings(ctx, format, api.FindingsFilter{
				Severity:  severityFilter,
				Status:    statusFilter,
				Scanner:   scannerFilter,
				ProjectID: projectID,
			})
			if err != nil {
				return err
			}

			w := os.Stdout
			if outputPath != "" {
				f, err := os.Create(outputPath)
				if err != nil {
					return err
				}
				defer f.Close()
				w = f
			}
			if _, err := w.Write(body); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "json", "Export format: json or csv")
	cmd.Flags().StringVar(&severityFilter, "severity", "", "Filter by severity (critical|high|medium|low)")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by finding status (e.g. open, fixed)")
	cmd.Flags().StringVar(&scannerFilter, "scanner", "", "Filter by scanner name")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Scope to a project UUID")
	cmd.Flags().StringVar(&outputPath, "output", "", "Write to this file instead of stdout")

	return cmd
}