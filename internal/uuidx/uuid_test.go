package uuidx

import (
	"regexp"
	"testing"
)

func TestNewV4(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, v); !ok {
		t.Fatalf("not a UUID v4: %q", v)
	}
}
