// Package severity classifies finding severities and computes fail-on thresholds.
//
// The numeric weights mirror henkaipan-action: critical=4, high=3, medium=2,
// low=1. Anything below the configured threshold is considered passing.
package severity

import (
	"fmt"
	"strings"
)

// Level is a normalized severity token accepted by HenKaiPan.
type Level string

const (
	Critical Level = "critical"
	High     Level = "high"
	Medium   Level = "medium"
	Low      Level = "low"
	None     Level = "none"
)

// All returns the severities in order from most to least severe.
func All() []Level { return []Level{Critical, High, Medium, Low} }

// Parse normalizes a free-form severity string ("HIGH", " High ", "high")
// into a canonical Level. Returns an error for unknown values.
func Parse(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return None, nil
	case "critical":
		return Critical, nil
	case "high":
		return High, nil
	case "medium":
		return Medium, nil
	case "low":
		return Low, nil
	case "none":
		return None, nil
	default:
		return "", fmt.Errorf("severity: unknown level %q (expected critical, high, medium, low, none)", s)
	}
}

// Weight maps a Level to its numeric weight, matching the action's
// SEVERITY_WEIGHT table so fail-on semantics are identical.
func Weight(l Level) int {
	switch l {
	case Critical:
		return 4
	case High:
		return 3
	case Medium:
		return 2
	case Low:
		return 1
	}
	return 0
}

// Counts is a tally of findings by severity. Anything not present is 0.
type Counts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// Total returns the sum of all severities.
func (c Counts) Total() int { return c.Critical + c.High + c.Medium + c.Low }

// MaxWeight returns the weight of the most severe finding present, or 0 if
// the counts are zero. This is the value compared against a fail-on threshold.
func (c Counts) MaxWeight() int {
	switch {
	case c.Critical > 0:
		return 4
	case c.High > 0:
		return 3
	case c.Medium > 0:
		return 2
	case c.Low > 0:
		return 1
	}
	return 0
}

// ExceedsThreshold reports whether any finding meets or exceeds the given
// threshold level.
func (c Counts) ExceedsThreshold(t Level) bool {
	if t == "" || t == None {
		return false
	}
	return c.MaxWeight() >= Weight(t)
}

// FromStrings tallies a slice of arbitrary severity strings into Counts.
// Unknown values are skipped (defensive against future API additions).
func FromStrings(sevs []string) Counts {
	var c Counts
	for _, s := range sevs {
		l, err := Parse(s)
		if err != nil {
			continue
		}
		switch l {
		case Critical:
			c.Critical++
		case High:
			c.High++
		case Medium:
			c.Medium++
		case Low:
			c.Low++
		}
	}
	return c
}