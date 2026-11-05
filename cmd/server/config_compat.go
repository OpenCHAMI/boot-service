// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// jwksURLEnv is the only JWKS setting the generated authentication reads.
const jwksURLEnv = "TOKENSMITH_JWKS_URL"

// setFlagDefault changes a generated flag's default without marking it as set,
// keeping --help accurate and letting config files override it.
func setFlagDefault(flags *pflag.FlagSet, name, value string) {
	flag := flags.Lookup(name)
	if flag == nil {
		return
	}
	if err := flag.Value.Set(value); err != nil {
		panic(fmt.Errorf("invalid default for --%s: %w", name, err))
	}
	flag.DefValue = value
}

// registerDeprecatedFlags keeps flags from before the feature loader working so
// that existing service definitions still start.
func registerDeprecatedFlags(flags *pflag.FlagSet) {
	flags.Bool("enable-auth", false, "Enable authentication")
	flags.String("storage-type", "file", "Storage backend")
	flags.String("jwks-endpoint", "", "JWKS URL for JWT validation")

	for name, message := range map[string]string{
		"enable-auth":   "use --auth-enabled, which also enforces JWTs on protected routes",
		"storage-type":  "the storage backend is selected by .fabrica.yaml at build time",
		"jwks-endpoint": "use --jwks-url or " + jwksURLEnv,
	} {
		if err := flags.MarkDeprecated(name, message); err != nil {
			panic(err)
		}
	}
}

// applyDeprecatedConfig maps settings from before the feature loader onto their
// replacements. It runs before the config struct is decoded.
//
// enable_auth must not become a no-op: a site that set it would keep running
// without the HSM token exchange and authentication it asked for, with nothing
// to say so. Mapping it instead fails closed, because authentication refuses to
// start without a JWKS URL.
func applyDeprecatedConfig(v *viper.Viper) error {
	if v.IsSet("enable_auth") {
		if v.IsSet("auth_enabled") {
			log.Printf("WARNING: ignoring deprecated enable_auth because auth_enabled is set")
		} else {
			log.Printf("WARNING: enable_auth is deprecated; use auth_enabled, which also enforces JWTs on protected routes")
			v.Set("auth_enabled", v.GetBool("enable_auth"))
		}
	}
	if v.IsSet("storage_type") {
		log.Printf("WARNING: ignoring deprecated storage_type; .fabrica.yaml selects the storage backend at build time")
	}
	return exportJWKSURL(v)
}

// exportJWKSURL makes the jwks_url setting effective, since the generated
// authentication reads only the environment. An explicit environment value wins.
func exportJWKSURL(v *viper.Viper) error {
	if os.Getenv(jwksURLEnv) != "" {
		return nil
	}

	url := strings.TrimSpace(v.GetString("jwks_url"))
	if url == "" {
		url = strings.TrimSpace(v.GetString("jwks_endpoint"))
		if url != "" {
			log.Printf("WARNING: jwks_endpoint is deprecated; use jwks_url or %s", jwksURLEnv)
		}
	}
	if url == "" {
		return nil
	}
	if err := os.Setenv(jwksURLEnv, url); err != nil {
		return fmt.Errorf("failed to set %s: %w", jwksURLEnv, err)
	}
	return nil
}

// rejectUnenforcedAuthSettings refuses settings the generated authentication
// accepts as flags but never reads. Running with an issuer or audience that is
// silently not checked would be weaker than the operator configured.
func rejectUnenforcedAuthSettings(v *viper.Viper, config Config) error {
	if !config.AuthEnabled {
		return nil
	}
	for _, key := range []string{"auth_non_enforcing", "jwt_public_key", "jwt_issuer", "jwt_audience"} {
		if v.IsSet(key) {
			return fmt.Errorf("%s is not supported by the generated authentication; remove it", key)
		}
	}
	return nil
}
