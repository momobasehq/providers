package airtel

import (
	"testing"

	mb "github.com/momobasehq/momobase/providers"
)

func TestStatus(t *testing.T) {
	if airtelStatus("TS") != mb.TxSucceeded || airtelStatus("DP") != mb.TxPending || airtelStatus("TF") != mb.TxFailed {
		t.Fatal("Airtel status mapping failed")
	}
}
