// Package scan contains the `henkaipan scan` subcommands.
package scan

import (
	"github.com/spf13/cobra"
)

// NewCmd returns the parent `scan` command with `run` and `status` attached.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Trigger scans and query scan status",
	}
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newStatusCmd())
	return cmd
}