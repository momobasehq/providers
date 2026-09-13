// Package marzpay implements MarzPay mobile-money collections and disbursements.
package marzpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	mb "github.com/momobasehq/momobase/providers"
	"github.com/momobasehq/providers/internal/configx"
	"github.com/momobasehq/providers/internal/httpx"
	"github.com/momobasehq/providers/internal/msisdn"
	"github.com/momobasehq/providers/internal/uuidx"
)

const liveURL = "https://wallet.wearemarz.com/api/v1"

type state struct {
	baseURL, apiKey, apiSecret, callbackURL, webhookSecret string
	caps                                                   []mb.Capability
}
type Provider struct {
	mu     sync.RWMutex
	s      state
	client *http.Client
}

func New(*slog.Logger) mb.PaymentProvider { return &Provider{client: httpx.Client()} }
func (p *Provider) Init(_ context.Context, c mb.ProviderConfig) error {
	if err := configx.Require(c, "api_key", "api_secret"); err != nil {
		return fmt.Errorf("marzpay: %w", err)
	}
	base := configx.String(c, "base_url")
	if base == "" {
		base = liveURL
	}
	s := state{baseURL: strings.TrimRight(base, "/"), apiKey: configx.String(c, "api_key"), apiSecret: configx.String(c, "api_secret"), callbackURL: configx.String(c, "callback_url"), webhookSecret: configx.String(c, "webhook_signing_secret"), caps: []mb.Capability{{ServiceType: mb.ServiceCollection, PaymentMethod: mb.PaymentMethodMomo}, {ServiceType: mb.ServiceCollection, PaymentMethod: mb.PaymentMethodCard}, {ServiceType: mb.ServiceDisbursement, PaymentMethod: mb.PaymentMethodMomo}}}
	if s.callbackURL != "" && s.webhookSecret == "" {
		return fmt.Errorf("marzpay: webhook_signing_secret is required when callback_url is set")
	}
	p.mu.Lock()
	p.s = s
	p.mu.Unlock()
	return nil
}
func (p *Provider) snapshot() state { p.mu.RLock(); defer p.mu.RUnlock(); return p.s }
func (p *Provider) Capabilities() []mb.Capability {
	s := p.snapshot()
	return append([]mb.Capability(nil), s.caps...)
}
func (p *Provider) ValidateRequest(_ context.Context, r *mb.PaymentRequest) error {
	switch r.PaymentMethod {
	case mb.PaymentMethodMomo:
		e, err := msisdn.E164(r.Account, r.Country)
		if err != nil {
			return err
		}
		r.Account = e
		return nil
	case mb.PaymentMethodCard:
		return nil
	default:
		return fmt.Errorf("marzpay: unsupported payment method %q", r.PaymentMethod)
	}
}
func auth(s state) map[string]string {
	return map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(s.apiKey+":"+s.apiSecret)), "Accept": "application/json"}
}

type createResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Transaction struct {
			UUID              string `json:"uuid"`
			Reference         string `json:"reference"`
			Status            string `json:"status"`
			ProviderReference any    `json:"provider_reference"`
		} `json:"transaction"`
		RedirectURL string `json:"redirect_url"`
	} `json:"data"`
}

func (p *Provider) pay(ctx context.Context, r mb.PaymentRequest, path string) (*mb.ProviderPaymentResponse, error) {
	s := p.snapshot()
	ref, err := uuidx.New()
	if err != nil {
		return nil, err
	}
	meta, _ := json.Marshal([]map[string]any{{"momobaseTransactionId": r.TransactionID}, {"applicationReference": r.Reference}})
	fields := map[string]string{"amount": mb.FormatAmountMinor(r.Amount, r.Currency), "country": strings.ToUpper(r.Country), "currency": strings.ToUpper(r.Currency), "reference": ref, "description": trim(configx.First(r.Description, r.Reference, "Momobase payment"), 255), "metadata": string(meta)}
	if r.PaymentMethod == mb.PaymentMethodMomo {
		fields["phone_number"] = r.Account
	} else if r.PaymentMethod == mb.PaymentMethodCard && path == "/collect-money" {
		fields["method"] = "card"
	}
	if s.callbackURL != "" {
		fields["callback_url"] = s.callbackURL
	}
	var out createResponse
	if err = httpx.Multipart(ctx, p.client, s.baseURL+path, auth(s), fields, &out); err != nil {
		return nil, err
	}
	if out.Data.Transaction.UUID == "" {
		return nil, fmt.Errorf("marzpay: %s", configx.First(out.Message, "missing transaction UUID"))
	}
	return &mb.ProviderPaymentResponse{ProviderReference: out.Data.Transaction.UUID, Status: marzStatus(out.Data.Transaction.Status), Message: configx.First(out.Message, out.Data.Transaction.Status), Raw: httpx.Map(out)}, nil
}
func (p *Provider) Collect(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	return p.pay(ctx, r, "/collect-money")
}
func (p *Provider) Disburse(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	if r.PaymentMethod != mb.PaymentMethodMomo {
		return nil, fmt.Errorf("marzpay: disbursement supports mobile money only")
	}
	return p.pay(ctx, r, "/send-money")
}

type amount struct {
	Raw      json.Number `json:"raw"`
	Currency string      `json:"currency"`
}
type webhook struct {
	EventType   string `json:"event_type"`
	Transaction struct {
		UUID        string `json:"uuid"`
		Reference   string `json:"reference"`
		Status      string `json:"status"`
		Amount      amount `json:"amount"`
		Provider    string `json:"provider"`
		PhoneNumber string `json:"phone_number"`
	} `json:"transaction"`
	Collection struct {
		PhoneNumber string `json:"phone_number"`
		Amount      amount `json:"amount"`
	} `json:"collection"`
	Disbursement struct {
		PhoneNumber string `json:"phone_number"`
		Amount      amount `json:"amount"`
	} `json:"disbursement"`
}

func decodeWebhook(body []byte) (webhook, map[string]any, error) {
	var w webhook
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if err := d.Decode(&w); err != nil {
		return w, nil, err
	}
	var raw map[string]any
	d = json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	_ = d.Decode(&raw)
	return w, raw, nil
}
func (p *Provider) QueryTransaction(ctx context.Context, ref, _ string) (*mb.ProviderTransactionStatus, error) {
	s := p.snapshot()
	b, err := httpx.Do(ctx, p.client, http.MethodGet, s.baseURL+"/transactions/"+ref, auth(s), "", nil)
	if err != nil {
		return nil, err
	}
	w, _, err := decodeWebhook(b)
	if err != nil {
		return nil, err
	}
	return &mb.ProviderTransactionStatus{ProviderReference: configx.First(w.Transaction.UUID, ref), Status: marzStatus(w.Transaction.Status), Message: configx.First(w.EventType, w.Transaction.Status)}, nil
}
func (p *Provider) VerifyWebhook(_ context.Context, body []byte, headers map[string]string) (*mb.ProviderWebhookEvent, error) {
	s := p.snapshot()
	if s.webhookSecret == "" {
		return nil, fmt.Errorf("marzpay: webhook signing is not configured")
	}
	ts := httpx.Header(headers, "X-MarzPay-Timestamp")
	sigHeader := httpx.Header(headers, "X-MarzPay-Signature")
	if ts == "" || sigHeader == "" {
		return nil, fmt.Errorf("marzpay: missing signature headers")
	}
	var received string
	for _, part := range strings.Split(sigHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "v1=") {
			received = strings.TrimPrefix(part, "v1=")
		}
	}
	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	_, _ = mac.Write([]byte(ts + "."))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if received == "" || !hmac.Equal([]byte(expected), []byte(received)) {
		return nil, fmt.Errorf("marzpay: invalid webhook signature")
	}
	w, raw, err := decodeWebhook(body)
	if err != nil {
		return nil, err
	}
	a := w.Transaction.Amount
	if a.Currency == "" {
		if w.Collection.Amount.Currency != "" {
			a = w.Collection.Amount
		} else {
			a = w.Disbursement.Amount
		}
	}
	var minor *int64
	if a.Raw.String() != "" {
		if n, e := mb.ParseAmountToMinor(a.Raw.String(), a.Currency); e == nil {
			minor = &n
		}
	}
	return &mb.ProviderWebhookEvent{ProviderReference: w.Transaction.UUID, Status: marzStatus(w.Transaction.Status), EventType: w.EventType, ExternalReference: w.Transaction.Reference, Amount: minor, Currency: a.Currency, Raw: raw}, nil
}
func marzStatus(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "completed", "successful", "success":
		return mb.TxSucceeded
	case "processing":
		return mb.TxProcessing
	case "pending", "sandbox":
		return mb.TxPending
	case "failed":
		return mb.TxFailed
	case "cancelled", "canceled":
		return mb.TxCancelled
	default:
		return mb.PaymentStatus(v)
	}
}
func trim(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

var _ mb.PaymentProvider = (*Provider)(nil)
var _ mb.Collector = (*Provider)(nil)
var _ mb.Disburser = (*Provider)(nil)
var _ mb.TransactionQuerier = (*Provider)(nil)
var _ mb.WebhookVerifier = (*Provider)(nil)
var _ mb.RequestValidator = (*Provider)(nil)
