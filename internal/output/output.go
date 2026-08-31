// Package output renders command results as table / json / yaml.
//
// The default is a human-friendly table (matching `gh` and `kubectl`); --json
// or --yaml switches to a serialized form suitable for piping into jq or
// loading in another tool.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"gopkg.in/yaml.v3"
)

// Format selects the renderer. Use ParseFormat to normalize user input.
type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	YAML  Format = "yaml"
)

// ParseFormat normalizes the --output flag value.
func ParseFormat(s string) (Format, error) {
	switch s {
	case "", "table":
		return Table, nil
	case "json":
		return JSON, nil
	case "yaml", "yml":
		return YAML, nil
	}
	return "", fmt.Errorf("output: unknown format %q (expected table, json, yaml)", s)
}

// WriteJSON serializes v as indented JSON to w.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// WriteYAML serializes v as YAML to w.
func WriteYAML(w io.Writer, v any) error {
	return yaml.NewEncoder(w).Encode(v)
}

// Tableize renders headers + rows as a borderless table to w.
// Auto-fit widths via WithMaxWidth.
func Tableize(w io.Writer, headers []string, rows [][]string) {
	tw := tablewriter.NewTable(w,
		tablewriter.WithHeader(headers),
		tablewriter.WithBorders(tw.Border{}),
	)
	for _, r := range rows {
		_ = tw.Append(r)
	}
	_ = tw.Render()
}

// Stdout is exposed so tests can redirect command output.
var Stdout io.Writer = os.Stdout