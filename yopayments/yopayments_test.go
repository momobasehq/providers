package yopayments

import (
	"testing"

	mb "github.com/momobasehq/momobase/providers"
)

func TestStatus(t *testing.T) {
	if yoStatus("SUCCEEDED", "") != mb.TxSucceeded || yoStatus("PENDING", "") != mb.TxPending || yoStatus("FAILED", "") != mb.TxFailed {
		t.Fatal("Yo! Payments status mapping failed")
	}
}
