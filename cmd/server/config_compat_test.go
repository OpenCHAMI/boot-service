// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// newConfigForTest resolves configYAML and args the way the server does. The
// flags are built here because serveCmd's are global to the test binary.
func newConfigForTest(t *testing.T, configYAML string, args ...string) (*viper.Viper, *Config) {
	t.Helper()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerGeneratedFeatureFlags(flags)
	setFlagDefault(flags, "auth-enabled", "false")
	registerDeprecatedFlags(flags)
	flags.String("tokensmith-url", "", "")
	if err := flags.Parse(args); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	if err := bindFlagsWithUnderscoreKeys(v, flags); err != nil {
		t.Fatal(err)
	}
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(configYAML)); err != nil {
		t.Fatal(err)
	}
	if err := applyDeprecatedConfig(v); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	if err := v.Unmarshal(cfg); err != nil {
		t.Fatal(err)
	}
	return v, cfg
}

func TestAuthConfigResolution(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		args []string
		want bool
	}{
		{name: "off by default"},
		{name: "auth_enabled key", yaml: "auth_enabled: true", want: true},
		{name: "auth-enabled flag", args: []string{"--auth-enabled"}, want: true},
		{name: "flag beats key", yaml: "auth_enabled: true", args: []string{"--auth-enabled=false"}},
		{name: "deprecated key still enables", yaml: "enable_auth: true", want: true},
		{name: "deprecated flag still enables", args: []string{"--enable-auth"}, want: true},
		{name: "deprecated false stays off", yaml: "enable_auth: false"},
		{name: "new key beats deprecated key", yaml: "enable_auth: true\nauth_enabled: false"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, cfg := newConfigForTest(t, tc.yaml, tc.args...)
			if cfg.AuthEnabled != tc.want {
				t.Fatalf("AuthEnabled = %v, want %v", cfg.AuthEnabled, tc.want)
			}
		})
	}
}

func TestDeprecatedAuthEnvEnablesAuthentication(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerGeneratedFeatureFlags(flags)
	setFlagDefault(flags, "auth-enabled", "false")
	registerDeprecatedFlags(flags)

	v := viper.New()
	if err := bindFlagsWithUnderscoreKeys(v, flags); err != nil {
		t.Fatal(err)
	}
	v.SetEnvPrefix("BOOT_SERVICE")
	v.AutomaticEnv()
	t.Setenv("BOOT_SERVICE_ENABLE_AUTH", "true")

	if err := applyDeprecatedConfig(v); err != nil {
		t.Fatal(err)
	}
	if !v.GetBool("auth_enabled") {
		t.Fatal("BOOT_SERVICE_ENABLE_AUTH must keep enabling authentication")
	}
}

func TestDefaultsKeepAuthenticationOptIn(t *testing.T) {
	_, cfg := newConfigForTest(t, "")
	if cfg.AuthEnabled {
		t.Fatal("authentication must stay off unless configured")
	}
	if cfg.TokenSmithURL != "" {
		t.Fatalf("tokensmith_url must have no default, got %q", cfg.TokenSmithURL)
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("data_dir default changed: %q", cfg.DataDir)
	}
	if help := serveCmd.Flags().Lookup("auth-enabled"); help == nil || help.DefValue != "false" {
		t.Fatalf("--auth-enabled must advertise a false default: %+v", help)
	}
}

func TestDeprecatedFlagsAreAcceptedAndHidden(t *testing.T) {
	for _, name := range []string{"enable-auth", "storage-type", "jwks-endpoint"} {
		flag := serveCmd.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("--%s must stay accepted so existing units still start", name)
		}
		if flag.Deprecated == "" {
			t.Fatalf("--%s must be marked deprecated", name)
		}
	}
}

func TestJWKSURLExport(t *testing.T) {
	tests := []struct {
		name string
		env  string
		yaml string
		want string
	}{
		{name: "unset"},
		{name: "jwks_url", yaml: "jwks_url: https://new/jwks", want: "https://new/jwks"},
		{name: "deprecated jwks_endpoint", yaml: "jwks_endpoint: https://old/jwks", want: "https://old/jwks"},
		{name: "jwks_url beats jwks_endpoint", yaml: "jwks_url: https://new/jwks\njwks_endpoint: https://old/jwks", want: "https://new/jwks"},
		{name: "environment wins", env: "https://env/jwks", yaml: "jwks_url: https://new/jwks", want: "https://env/jwks"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// t.Setenv restores the variable that exportJWKSURL may set.
			t.Setenv(jwksURLEnv, tc.env)
			newConfigForTest(t, tc.yaml)
			if got := os.Getenv(jwksURLEnv); got != tc.want {
				t.Fatalf("%s = %q, want %q", jwksURLEnv, got, tc.want)
			}
		})
	}
}

func TestRejectUnenforcedAuthSettings(t *testing.T) {
	for _, key := range []string{"auth_non_enforcing: true", "jwt_public_key: x", "jwt_issuer: x", "jwt_audience: x"} {
		t.Run(key, func(t *testing.T) {
			v, cfg := newConfigForTest(t, key+"\nauth_enabled: true")
			if err := rejectUnenforcedAuthSettings(v, *cfg); err == nil {
				t.Fatal("expected an error while authentication is enabled")
			}
			v, cfg = newConfigForTest(t, key)
			if err := rejectUnenforcedAuthSettings(v, *cfg); err != nil {
				t.Fatalf("settings are harmless while authentication is off: %v", err)
			}
		})
	}
}
