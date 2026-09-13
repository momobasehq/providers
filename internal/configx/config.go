package configx

import (
	"fmt"
	"strings"

	mb "github.com/momobasehq/momobase/providers"
)

// Require reports the first of keys that is missing from c.
func Require(c mb.ProviderConfig, keys ...string) error {
	for _, key := range keys {
		if mb.ConfigString(c, key) == "" {
			return fmt.Errorf("missing provider config %q", key)
		}
	}
	return nil
}

// Environment returns the configured environment, defaulting to sandbox.
func Environment(c mb.ProviderConfig) string {
	env := strings.ToLower(mb.ConfigString(c, "environment"))
	if env == "" {
		return "sandbox"
	}
	return env
}
