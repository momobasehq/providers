package flutterwave

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	mb "github.com/momobasehq/momobase/providers"
)

func TestVerifyWebhook(t *testing.T) {
	p := New(nil).(*Provider)
	if err := p.Init(context.Background(), mb.ProviderConfig{"client_id": "id", "client_secret": "secret", "webhook_secret": "whsec"}); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"type":"charge.completed","data":{"id":"chg_1","amount":200,"currency":"UGX","reference":"ref-1","status":"succeeded","payment_method":{"mobile_money":{"country_code":"256","phone_number":"700000000"}}}}`)
	mac := hmac.New(sha256.New, []byte("whsec"))
	_, _ = mac.Write(body)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	event, err := p.VerifyWebhook(context.Background(), body, map[string]string{"flutterwave-signature": sig})
	if err != nil {
		t.Fatal(err)
	}
	if event.ProviderReference != "chg_1" || event.Status != mb.TxSucceeded || event.Account != "+256700000000" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestReferenceBounds(t *testing.T) {
	got := flutterwaveReference("bad ref/@-12345678901234567890123456789012345678901234567890")
	if len(got) < 6 || len(got) > 42 {
		t.Fatalf("invalid reference length %d: %q", len(got), got)
	}
}
