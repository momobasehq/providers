package configx

import (
	"fmt"
	"strings"

	mb "github.com/momobasehq/momobase/providers"
)

func String(c mb.ProviderConfig, key string) string { return mb.ConfigString(c, key) }

func Require(c mb.ProviderConfig, keys ...string) error {
	for _, key := range keys {
		if String(c, key) == "" {
			return fmt.Errorf("missing provider config %q", key)
		}
	}
	return nil
}

func Environment(c mb.ProviderConfig) string {
	env := strings.ToLower(String(c, "environment"))
	if env == "" {
		return "sandbox"
	}
	return env
}

func First(values ...string) string { return mb.First(values...) }
