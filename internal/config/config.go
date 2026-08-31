// Package config loads CLI configuration with the precedence required by the
// epic: explicit flags override HENKAIPAN_* environment variables, which in
// turn override built-in defaults.
//
// The API key is treated as a secret throughout: it is read into a typed
// SecretValue wrapper that masks itself whenever it is rendered via
// fmt.Stringer / %s / %v, so accidental debug prints cannot leak it.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Defaults that match the epic and the existing henkaipan-action contract.
const (
	DefaultAPIURL = "https://henkaipan.dyallab.com.ar"
	UserAgent     = "henkaipan-cli/0.1.0"
)

// SecretValue wraps a string that should never appear in plain-text output.
// It implements fmt.Stringer / fmt.GoStringer to always render as "***".
type SecretValue string

// String returns the masked representation.
func (s SecretValue) String() string { return "***" }

// GoString returns the masked representation.
func (s SecretValue) GoString() string { return "***" }

// Value returns the raw underlying secret. Use only when actually transmitting
// it (e.g. building an HTTP header).
func (s SecretValue) Value() string { return string(s) }

// Config is the resolved runtime configuration shared across all commands.
type Config struct {
	APIURL               string      `mapstructure:"api_url"`
	APIKey               SecretValue `mapstructure:"api_key"`
	CFAccessClientID     string      `mapstructure:"cf_access_client_id"`
	CFAccessClientSecret SecretValue `mapstructure:"cf_access_client_secret"`
	Output               string      `mapstructure:"output"`
	TimeoutSeconds       int         `mapstructure:"timeout_seconds"`
}

// FlagSet returns the persistent flags shared across the CLI; this is the
// single place to add or rename a config field and have it flow through every
// subcommand. The returned flags are attached to the root command by cli.NewRootCmd.
func FlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("henkaipan", pflag.ContinueOnError)

	fs.String("api-url", DefaultAPIURL, "Base URL of the HenKaiPan API")
	fs.String("api-key", "", "API key (X-API-Key header). Prefer HENKAIPAN_API_KEY env var")
	fs.String("output", "table", "Output format: table, json, yaml")
	fs.Int("timeout", 60, "HTTP request timeout in seconds")
	fs.String("cf-access-client-id", "", "Cloudflare Access Service Token Client ID")
	fs.String("cf-access-client-secret", "", "Cloudflare Access Service Token Client Secret")

	return fs
}

// Bind wires viper to the given flag set so the precedence (flag > env >
// default) holds for every subcommand. Call this once after the root command
// has registered the flag set returned by FlagSet.
func Bind(v *viper.Viper, fs *pflag.FlagSet) {
	v.SetEnvPrefix("HENKAIPAN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	_ = v.BindPFlag("api_url", fs.Lookup("api-url"))
	_ = v.BindPFlag("api_key", fs.Lookup("api-key"))
	_ = v.BindPFlag("output", fs.Lookup("output"))
	_ = v.BindPFlag("timeout_seconds", fs.Lookup("timeout"))
	_ = v.BindPFlag("cf_access_client_id", fs.Lookup("cf-access-client-id"))
	_ = v.BindPFlag("cf_access_client_secret", fs.Lookup("cf-access-client-secret"))

	v.SetDefault("api_url", DefaultAPIURL)
	v.SetDefault("output", "table")
	v.SetDefault("timeout_seconds", 60)
}

// Load reads the bound configuration into a typed Config struct.
func Load(v *viper.Viper) (*Config, error) {
	c := &Config{}
	if err := v.Unmarshal(c); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	c.APIURL = strings.TrimRight(c.APIURL, "/")
	return c, nil
}