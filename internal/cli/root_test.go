package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandTree(t *testing.T) {
	root := NewRootCmd()
	want := []string{"scan", "findings", "version"}
	for _, name := range want {
		found := false
		for _, c := range root.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q", name)
		}
	}
}

func TestVersionSubcommand(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"version"})
	var out bytes.Buffer
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.HasPrefix(got, "henkaipan-cli/") {
		t.Errorf("version output = %q", got)
	}
}