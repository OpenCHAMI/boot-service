// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var declarativeAlgorithms = []string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512"}

type declarativeClaims struct {
	jwt.RegisteredClaims
	Scope declarativeScopes `json:"scope"`
}

type declarativeScopes []string

func (s *declarativeScopes) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return ErrDeclarativeInvalid
	}
	var values []string
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return ErrDeclarativeInvalid
		}
		for _, c := range value {
			if c != ' ' && !validScopeCharacter(c) {
				return ErrDeclarativeInvalid
			}
		}
		values = strings.Fields(value)
	} else if data[0] == '[' {
		if err := json.Unmarshal(data, &values); err != nil {
			return ErrDeclarativeInvalid
		}
		for _, value := range values {
			if value == "" {
				return ErrDeclarativeInvalid
			}
			for _, c := range value {
				if !validScopeCharacter(c) {
					return ErrDeclarativeInvalid
				}
			}
		}
	} else {
		return ErrDeclarativeInvalid
	}
	*s = values
	return nil
}

func validScopeCharacter(c rune) bool {
	return c == 0x21 || (c >= 0x23 && c <= 0x5b) || (c >= 0x5d && c <= 0x7e)
}

// Reject ambiguous JWT objects before jwt/v5 decodes them with last-key-wins semantics.
func validJWTObjects(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	for i := 0; i < 2; i++ {
		data, err := base64.RawURLEncoding.Strict().DecodeString(parts[i])
		if err != nil || !uniqueJSONObject(data, i == 0) {
			return false
		}
		if i == 1 {
			var claims map[string]json.RawMessage
			if json.Unmarshal(data, &claims) != nil {
				return false
			}
			// Unknown extension claims are ignored; security claims must use exact names.
			for name := range claims {
				for _, canonical := range [...]string{"scope", "iss", "aud", "exp", "nbf", "iat", "sub", "jti"} {
					if name != canonical && strings.EqualFold(name, canonical) {
						return false
					}
				}
			}
		}
	}
	return true
}

func uniqueJSONObject(data []byte, header bool) bool {
	d := json.NewDecoder(bytes.NewReader(data))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	seen := make(map[string]struct{})
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return false
		}
		if _, duplicate := seen[name]; duplicate {
			return false
		}
		if header && (name == "crit" || name == "b64") {
			return false
		}
		seen[name] = struct{}{}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return false
		}
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return false
	}
	_, err = d.Token()
	return err == io.EOF
}
