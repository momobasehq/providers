package marzpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	mb "github.com/momobasehq/momobase/providers"
)

func TestVerifyWebhook(t *testing.T) {
	p := New(nil).(*Provider)
	if err := p.Init(context.Background(), mb.ProviderConfig{"api_key": "key", "api_secret": "secret", "webhook_signing_secret": "whsec"}); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"event_type":"collection.completed","transaction":{"uuid":"tx-1","reference":"external-1","status":"completed","amount":{"raw":1000,"currency":"UGX"}},"collection":{"phone_number":"+256700000000"}}`)
	ts := "1712345678"
	mac := hmac.New(sha256.New, []byte("whsec"))
	_, _ = mac.Write([]byte(ts + "."))
	_, _ = mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	event, err := p.VerifyWebhook(context.Background(), body, map[string]string{"X-MarzPay-Timestamp": ts, "X-MarzPay-Signature": "t=" + ts + ",v1=" + sig})
	if err != nil {
		t.Fatal(err)
	}
	if event.ProviderReference != "tx-1" || event.Status != mb.TxSucceeded || event.Amount == nil || *event.Amount != 1000 {
		t.Fatalf("unexpected event: %#v", event)
	}
}
