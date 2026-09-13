package mtn

import (
	"testing"

	mb "github.com/momobasehq/momobase/providers"
)

func TestStatus(t *testing.T) {
	if status("SUCCESSFUL") != mb.TxSucceeded || status("PENDING") != mb.TxPending || status("FAILED") != mb.TxFailed {
		t.Fatal("MTN status mapping failed")
	}
	// PENDING is an override; the rest is delegated to mb.PaymentStatus.
	if status("CANCELLED") != mb.TxCancelled || status("EXPIRED") != mb.TxExpired {
		t.Fatal("MTN status delegation failed")
	}
}
