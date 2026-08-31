// Package findings contains the `henkaipan findings` subcommands.
package findings

import "github.com/spf13/cobra"

// NewCmd returns the parent `findings` command with `list` and `export` attached.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "findings",
		Short: "List and export findings from HenKaiPan",
	}
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newExportCmd())
	return cmd
}