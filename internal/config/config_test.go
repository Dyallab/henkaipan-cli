package config

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// freshViper creates a brand-new viper + flag set pair so each test has
// independent precedence state. This mirrors how cli.NewRootCmd wires them
// together once per process.
func freshViper() (*viper.Viper, *pflag.FlagSet) {
	v := viper.New()
	fs := FlagSet()
	Bind(v, fs)
	return v, fs
}

func TestDefaultAPIURL(t *testing.T) {
	v, _ := freshViper()
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != DefaultAPIURL {
		t.Errorf("default api-url = %q, want %q", cfg.APIURL, DefaultAPIURL)
	}
}

func TestEnvOverridesDefault(t *testing.T) {
	t.Setenv("HENKAIPAN_API_URL", "https://example.test")

	v, _ := freshViper()
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://example.test" {
		t.Errorf("env override failed; got %q", cfg.APIURL)
	}
}

func TestSecretValueMasks(t *testing.T) {
	s := SecretValue("hkp_super_secret")
	if s.String() != "***" {
		t.Errorf("String() should mask; got %q", s.String())
	}
	if s.GoString() != "***" {
		t.Errorf("GoString() should mask; got %q", s.GoString())
	}
	if s.Value() != "hkp_super_secret" {
		t.Errorf("Value() should return raw; got %q", s.Value())
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	t.Setenv("HENKAIPAN_API_URL", "https://env.test")

	// Re-parse a fresh flag set with the CLI flag set, then re-bind viper
	// so the parsed flag value is visible.
	parsed := FlagSet()
	if err := parsed.Parse([]string{"--api-url=https://flag.test"}); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	Bind(v, parsed)
	cfg, _ := Load(v)
	if cfg.APIURL != "https://flag.test" {
		t.Errorf("flag should beat env; got %q", cfg.APIURL)
	}
}

func TestTrailingSlashTrimmed(t *testing.T) {
	t.Setenv("HENKAIPAN_API_URL", "https://example.test/")

	v, _ := freshViper()
	cfg, _ := Load(v)
	if strings.HasSuffix(cfg.APIURL, "/") {
		t.Errorf("trailing slash not trimmed: %q", cfg.APIURL)
	}
}