package mtn

import (
	"testing"

	mb "github.com/momobasehq/momobase/providers"
)

func TestStatus(t *testing.T) {
	if status("SUCCESSFUL") != mb.TxSucceeded || status("PENDING") != mb.TxPending || status("FAILED") != mb.TxFailed {
		t.Fatal("MTN status mapping failed")
	}
}
