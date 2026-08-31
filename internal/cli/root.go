// Package cli assembles the cobra command tree for henkaipan.
//
// Layout mirrors the action's vocabulary so muscle memory transfers:
//
//	henkaipan scan run       — POST /api/v1/scans/external
//	henkaipan scan status    — GET  /api/v1/scans/{id}/status  (with --wait)
//	henkaipan findings list  — GET  /api/findings
//	henkaipan findings export — GET /api/findings/export
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dyallab/henkaipan-cli/internal/cli/findings"
	"github.com/dyallab/henkaipan-cli/internal/cli/scan"
	"github.com/dyallab/henkaipan-cli/internal/config"
)

// NewRootCmd builds the root command with persistent flags wired up.
func NewRootCmd() *cobra.Command {
	flags := config.FlagSet()

	root := &cobra.Command{
		Use:   "henkaipan",
		Short: "HenKaiPan CLI — drive HenKaiPan from any terminal or script",
		Long: `henkaipan is the standalone, client-agnostic companion to the henkaipan-action
GitHub Action. It talks to the HenKaiPan REST API using the same X-API-Key
authentication and the same scan/findings endpoints, so it works against any
HenKaiPan instance (self-hosted or cloud) from any shell, script, or CI step.

Common usage:

  henkaipan scan run --repo-url https://github.com/owner/repo --scanners all
  henkaipan scan status <scan-id> --wait
  henkaipan findings list --severity high --project-id <uuid>
  henkaipan findings export --format json --severity critical`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			v := viper.GetViper()
			config.Bind(v, flags)
			return nil
		},
	}

	root.PersistentFlags().AddFlagSet(flags)

	root.AddCommand(scan.NewCmd())
	root.AddCommand(findings.NewCmd())

	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the henkaipan CLI version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), config.UserAgent)
		},
	})

	return root
}