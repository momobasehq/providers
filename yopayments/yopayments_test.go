package yopayments

import (
	"testing"

	mb "github.com/momobasehq/momobase/providers"
)

func TestStatus(t *testing.T) {
	if yoStatus("SUCCEEDED", "") != mb.TxSucceeded || yoStatus("PENDING", "") != mb.TxPending || yoStatus("FAILED", "") != mb.TxFailed {
		t.Fatal("Yo! Payments status mapping failed")
	}
	// Yo!-specific codes, then the vocabularies delegated to mb.PaymentStatus.
	if yoStatus("", "OK") != mb.TxSucceeded || yoStatus("INDETERMINATE", "") != mb.TxProcessing || yoStatus("FAIL", "") != mb.TxFailed {
		t.Fatal("Yo! Payments status code mapping failed")
	}
	if yoStatus("CANCELLED", "") != mb.TxCancelled || yoStatus("SUCCESSFUL", "") != mb.TxSucceeded {
		t.Fatal("Yo! Payments status delegation failed")
	}
}
