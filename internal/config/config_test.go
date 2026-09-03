package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func freshViper() (*viper.Viper, *pflag.FlagSet) {
	v := viper.New()
	fs := FlagSet()
	Bind(v, fs)
	return v, fs
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingConfigErrors(t *testing.T) {
	v, _ := freshViper()
	if err := InitConfig(v, "/tmp/does-not-exist-henkaipan.toml"); err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestFileLoad(t *testing.T) {
	cfgFile := writeTempConfig(t, "api_url = \"https://file.test\"\noutput = \"json\"\ntimeout_seconds = 60\n")
	v, _ := freshViper()
	if err := InitConfig(v, cfgFile); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://file.test" {
		t.Errorf("file load failed; got %q", cfg.APIURL)
	}
	if cfg.Output != "json" {
		t.Errorf("output from file = %q, want json", cfg.Output)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	cfgFile := writeTempConfig(t, "api_url = \"https://file.test\"\noutput = \"table\"\ntimeout_seconds = 60\n")
	t.Setenv("HENKAIPAN_API_URL", "https://env.test")
	t.Setenv("HENKAIPAN_OUTPUT", "yaml")

	v, _ := freshViper()
	if err := InitConfig(v, cfgFile); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://env.test" {
		t.Errorf("env should beat file; got %q", cfg.APIURL)
	}
	if cfg.Output != "yaml" {
		t.Errorf("env output should beat file; got %q", cfg.Output)
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
	cfgFile := writeTempConfig(t, "api_url = \"https://file.test\"\noutput = \"table\"\ntimeout_seconds = 60\n")
	t.Setenv("HENKAIPAN_API_URL", "https://env.test")

	parsed := FlagSet()
	if err := parsed.Parse([]string{"--api-url=https://flag.test"}); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	Bind(v, parsed)
	if err := InitConfig(v, cfgFile); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://flag.test" {
		t.Errorf("flag should beat env; got %q", cfg.APIURL)
	}
}

func TestTrailingSlashTrimmed(t *testing.T) {
	cfgFile := writeTempConfig(t, "output = \"table\"\ntimeout_seconds = 60\n")
	t.Setenv("HENKAIPAN_API_URL", "https://example.test/")

	v, _ := freshViper()
	if err := InitConfig(v, cfgFile); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(cfg.APIURL, "/") {
		t.Errorf("trailing slash not trimmed: %q", cfg.APIURL)
	}
}

func TestFlagOverridesFile(t *testing.T) {
	cfgFile := writeTempConfig(t, "api_url = \"https://file.test\"\noutput = \"table\"\ntimeout_seconds = 60\n")
	parsed := FlagSet()
	if err := parsed.Parse([]string{"--api-url=https://flag.test", "--output=json"}); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	Bind(v, parsed)
	if err := InitConfig(v, cfgFile); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://flag.test" {
		t.Errorf("flag should beat file; got %q", cfg.APIURL)
	}
	if cfg.Output != "json" {
		t.Errorf("flag output should beat file; got %q", cfg.Output)
	}
}