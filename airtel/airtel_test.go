package airtel

import (
	"testing"

	mb "github.com/momobasehq/momobase/providers"
)

func TestStatus(t *testing.T) {
	if airtelStatus("TS") != mb.TxSucceeded || airtelStatus("DP") != mb.TxPending || airtelStatus("TIP") != mb.TxProcessing || airtelStatus("TF") != mb.TxFailed {
		t.Fatal("Airtel status code mapping failed")
	}
	// Vocabularies delegated to mb.PaymentStatus.
	if airtelStatus("SUCCESSFUL") != mb.TxSucceeded || airtelStatus("PROCESSING") != mb.TxProcessing || airtelStatus("FAILED") != mb.TxFailed {
		t.Fatal("Airtel status delegation failed")
	}
}
