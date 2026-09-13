// Package flutterwave implements Flutterwave v4 mobile-money collections and payouts.
package flutterwave

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	mb "github.com/momobasehq/momobase/providers"
	"github.com/momobasehq/providers/internal/configx"
	"github.com/momobasehq/providers/internal/httpx"
	"github.com/momobasehq/providers/internal/msisdn"
	"github.com/momobasehq/providers/internal/token"
	"github.com/momobasehq/providers/internal/uuidx"
)

const (
	tokenURL   = "https://idp.flutterwave.com/realms/flutterwave/protocol/openid-connect/token"
	sandboxURL = "https://developersandbox-api.flutterwave.com"
	liveURL    = "https://f4bexperience.flutterwave.com"
)

type state struct {
	baseURL       string
	clientID      string
	clientSecret  string
	redirectURL   string
	callbackURL   string
	webhookSecret string
	caps          []mb.Capability
}

type Provider struct {
	mu     sync.RWMutex
	s      state
	client *http.Client
	tokens token.Cache
}

func New(*slog.Logger) mb.PaymentProvider { return &Provider{client: httpx.Client()} }

func (p *Provider) Init(_ context.Context, c mb.ProviderConfig) error {
	if err := configx.Require(c, "client_id", "client_secret", "webhook_secret"); err != nil {
		return fmt.Errorf("flutterwave: %w", err)
	}
	base := configx.String(c, "base_url")
	if base == "" {
		if configx.Environment(c) == "production" {
			base = liveURL
		} else {
			base = sandboxURL
		}
	}
	p.mu.Lock()
	p.s = state{
		baseURL:       strings.TrimRight(base, "/"),
		clientID:      configx.String(c, "client_id"),
		clientSecret:  configx.String(c, "client_secret"),
		redirectURL:   configx.String(c, "redirect_url"),
		callbackURL:   configx.String(c, "callback_url"),
		webhookSecret: configx.String(c, "webhook_secret"),
		caps: []mb.Capability{
			{ServiceType: mb.ServiceCollection, PaymentMethod: mb.PaymentMethodMomo},
			{ServiceType: mb.ServiceDisbursement, PaymentMethod: mb.PaymentMethodMomo},
		},
	}
	p.mu.Unlock()
	p.tokens.Reset()
	return nil
}

func (p *Provider) snapshot() state {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.s
}

func (p *Provider) Capabilities() []mb.Capability {
	s := p.snapshot()
	return append([]mb.Capability(nil), s.caps...)
}

func (p *Provider) ValidateRequest(_ context.Context, r *mb.PaymentRequest) error {
	if r.PaymentMethod != mb.PaymentMethodMomo {
		return fmt.Errorf("flutterwave: this adapter exposes mobile money only")
	}
	if strings.TrimSpace(r.Scheme) == "" {
		return fmt.Errorf("flutterwave: scheme is required and must name the mobile-money network")
	}
	if msisdn.CountryCode(r.Country) == "" {
		return fmt.Errorf("flutterwave: unsupported or missing country %q", r.Country)
	}
	e164, err := msisdn.E164(r.Account, r.Country)
	if err != nil {
		return fmt.Errorf("flutterwave: %w", err)
	}
	r.Account = e164
	r.Scheme = strings.ToUpper(strings.TrimSpace(r.Scheme))
	return nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func (p *Provider) accessToken(ctx context.Context) (string, error) {
	s := p.snapshot()
	return p.tokens.Get(ctx, func(ctx context.Context) (string, time.Duration, error) {
		var out tokenResponse
		err := httpx.Form(ctx, p.client, http.MethodPost, tokenURL, nil, url.Values{
			"client_id":     {s.clientID},
			"client_secret": {s.clientSecret},
			"grant_type":    {"client_credentials"},
		}, &out)
		if err != nil {
			return "", 0, fmt.Errorf("flutterwave authenticate: %w", err)
		}
		if out.AccessToken == "" {
			return "", 0, fmt.Errorf("flutterwave authenticate: empty access token")
		}
		return out.AccessToken, time.Duration(out.ExpiresIn) * time.Second, nil
	})
}

func requestHeaders(accessToken string) (map[string]string, error) {
	trace, err := uuidx.New()
	if err != nil {
		return nil, err
	}
	idem, err := uuidx.New()
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"Authorization":     "Bearer " + accessToken,
		"Accept":            "application/json",
		"X-Trace-Id":        trace,
		"X-Idempotency-Key": idem,
	}, nil
}

type apiResponse[T any] struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    T      `json:"data"`
	Error   any    `json:"error"`
}

type customerData struct {
	ID string `json:"id"`
}

type paymentMethodData struct {
	ID string `json:"id"`
}

type chargeData struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Reference string `json:"reference"`
	Currency  string `json:"currency"`
	Amount    any    `json:"amount"`
}

type transferData struct {
	ID                  string `json:"id"`
	Status              string `json:"status"`
	Reference           string `json:"reference"`
	SourceCurrency      string `json:"source_currency"`
	DestinationCurrency string `json:"destination_currency"`
	Amount              struct {
		Value     any    `json:"value"`
		AppliesTo string `json:"applies_to"`
	} `json:"amount"`
}

func (p *Provider) post(ctx context.Context, endpoint string, token string, in, out any) error {
	headers, err := requestHeaders(token)
	if err != nil {
		return err
	}
	if err := httpx.JSON(ctx, p.client, http.MethodPost, endpoint, headers, in, out); err != nil {
		return err
	}
	return nil
}

func (p *Provider) get(ctx context.Context, endpoint string, token string, out any) error {
	headers, err := requestHeaders(token)
	if err != nil {
		return err
	}
	return httpx.JSON(ctx, p.client, http.MethodGet, endpoint, headers, nil, out)
}

func (p *Provider) Collect(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	if strings.TrimSpace(r.Email) == "" {
		return nil, fmt.Errorf("flutterwave: email is required for mobile-money collection")
	}
	s := p.snapshot()
	tok, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	cc := msisdn.CountryCode(r.Country)
	local, err := msisdn.Local(r.Account, r.Country)
	if err != nil {
		return nil, err
	}
	first, last := splitName(r.Name)

	customerReq := map[string]any{
		"email": r.Email,
		"name":  map[string]string{"first": first, "last": last},
		"phone": map[string]string{"country_code": cc, "number": local},
		"meta":  r.Metadata,
	}
	var customer apiResponse[customerData]
	if err := p.post(ctx, s.baseURL+"/customers", tok, customerReq, &customer); err != nil {
		return nil, fmt.Errorf("flutterwave create customer: %w", err)
	}
	if customer.Data.ID == "" {
		return nil, fmt.Errorf("flutterwave create customer: %s", configx.First(customer.Message, "missing customer ID"))
	}

	paymentReq := map[string]any{
		"type": "mobile_money",
		"mobile_money": map[string]string{
			"country_code": cc,
			"network":      r.Scheme,
			"phone_number": local,
		},
	}
	var paymentMethod apiResponse[paymentMethodData]
	if err := p.post(ctx, s.baseURL+"/payment-methods", tok, paymentReq, &paymentMethod); err != nil {
		return nil, fmt.Errorf("flutterwave create payment method: %w", err)
	}
	if paymentMethod.Data.ID == "" {
		return nil, fmt.Errorf("flutterwave create payment method: %s", configx.First(paymentMethod.Message, "missing payment-method ID"))
	}

	chargeReq := map[string]any{
		"reference":         flutterwaveReference(r.TransactionID),
		"currency":          strings.ToUpper(r.Currency),
		"customer_id":       customer.Data.ID,
		"payment_method_id": paymentMethod.Data.ID,
		"amount":            amountNumber(r.Amount, r.Currency),
		"meta": map[string]any{
			"momobase_transaction_id": r.TransactionID,
			"application_reference":   r.Reference,
		},
	}
	if s.redirectURL != "" {
		chargeReq["redirect_url"] = s.redirectURL
	}
	var charge apiResponse[chargeData]
	if err := p.post(ctx, s.baseURL+"/charges", tok, chargeReq, &charge); err != nil {
		return nil, fmt.Errorf("flutterwave create charge: %w", err)
	}
	if charge.Data.ID == "" {
		return nil, fmt.Errorf("flutterwave create charge: %s", configx.First(charge.Message, "missing charge ID"))
	}
	return &mb.ProviderPaymentResponse{
		ProviderReference: charge.Data.ID,
		Status:            flutterwaveStatus(charge.Data.Status),
		Message:           configx.First(charge.Message, charge.Data.Status),
		Raw: map[string]any{
			"customer":       httpx.Map(customer),
			"payment_method": httpx.Map(paymentMethod),
			"charge":         httpx.Map(charge),
		},
	}, nil
}

func (p *Provider) Disburse(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	s := p.snapshot()
	tok, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	reference := flutterwaveReference(r.TransactionID)
	in := map[string]any{
		"action":    "instant",
		"type":      "mobile_money",
		"reference": reference,
		"narration": truncate(configx.First(r.Description, r.Reference, "Momobase payout"), 180),
		"payment_instruction": map[string]any{
			"source_currency":      strings.ToUpper(r.Currency),
			"destination_currency": strings.ToUpper(r.Currency),
			"amount": map[string]any{
				"applies_to": "destination_currency",
				"value":      amountNumber(r.Amount, r.Currency),
			},
			"recipient": map[string]any{
				"name": configx.First(r.Name, "Momobase recipient"),
				"mobile_money": map[string]string{
					"network": r.Scheme,
					"msisdn":  strings.TrimPrefix(r.Account, "+"),
				},
			},
		},
	}
	if s.callbackURL != "" {
		in["callback_url"] = s.callbackURL
	}
	var out apiResponse[transferData]
	if err := p.post(ctx, s.baseURL+"/direct-transfers", tok, in, &out); err != nil {
		return nil, fmt.Errorf("flutterwave create mobile-money transfer: %w", err)
	}
	if out.Data.ID == "" {
		return nil, fmt.Errorf("flutterwave create mobile-money transfer: %s", configx.First(out.Message, "missing transfer ID"))
	}
	return &mb.ProviderPaymentResponse{
		ProviderReference: out.Data.ID,
		Status:            flutterwaveStatus(out.Data.Status),
		Message:           configx.First(out.Message, out.Data.Status),
		Raw:               httpx.Map(out),
	}, nil
}

func (p *Provider) QueryTransaction(ctx context.Context, ref, _ string) (*mb.ProviderTransactionStatus, error) {
	s := p.snapshot()
	tok, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(ref, "trf_") {
		return p.queryTransfer(ctx, s, tok, ref)
	}
	if strings.HasPrefix(ref, "chg_") {
		return p.queryCharge(ctx, s, tok, ref)
	}
	if status, err := p.queryCharge(ctx, s, tok, ref); err == nil {
		return status, nil
	}
	return p.queryTransfer(ctx, s, tok, ref)
}

func (p *Provider) queryCharge(ctx context.Context, s state, tok, ref string) (*mb.ProviderTransactionStatus, error) {
	var out apiResponse[chargeData]
	if err := p.get(ctx, s.baseURL+"/charges/"+url.PathEscape(ref), tok, &out); err != nil {
		return nil, fmt.Errorf("flutterwave query charge: %w", err)
	}
	return &mb.ProviderTransactionStatus{ProviderReference: configx.First(out.Data.ID, ref), Status: flutterwaveStatus(out.Data.Status), Message: configx.First(out.Message, out.Data.Status)}, nil
}

func (p *Provider) queryTransfer(ctx context.Context, s state, tok, ref string) (*mb.ProviderTransactionStatus, error) {
	var out apiResponse[transferData]
	if err := p.get(ctx, s.baseURL+"/transfers/"+url.PathEscape(ref), tok, &out); err != nil {
		return nil, fmt.Errorf("flutterwave query transfer: %w", err)
	}
	return &mb.ProviderTransactionStatus{ProviderReference: configx.First(out.Data.ID, ref), Status: flutterwaveStatus(out.Data.Status), Message: configx.First(out.Message, out.Data.Status)}, nil
}

func (p *Provider) HealthCheck(ctx context.Context) error {
	_, err := p.accessToken(ctx)
	return err
}

func (p *Provider) VerifyWebhook(_ context.Context, body []byte, headers map[string]string) (*mb.ProviderWebhookEvent, error) {
	s := p.snapshot()
	sig := httpx.Header(headers, "flutterwave-signature")
	if sig == "" {
		return nil, fmt.Errorf("flutterwave: missing flutterwave-signature header")
	}
	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	_, _ = mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(sig))) {
		return nil, fmt.Errorf("flutterwave: invalid webhook signature")
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var payload map[string]any
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("flutterwave: decode webhook: %w", err)
	}
	data, _ := payload["data"].(map[string]any)
	if len(data) == 0 {
		return nil, fmt.Errorf("flutterwave: webhook is missing data")
	}
	providerRef := stringValue(data["id"])
	if providerRef == "" {
		return nil, fmt.Errorf("flutterwave: webhook is missing transaction id")
	}
	currency := configx.First(stringValue(data["currency"]), stringValue(data["destination_currency"]), stringValue(data["source_currency"]))
	amountText := numberValue(data["amount"])
	if m, ok := data["amount"].(map[string]any); ok {
		amountText = numberValue(m["value"])
	}
	var minor *int64
	if amountText != "" && currency != "" {
		if n, err := mb.ParseAmountToMinor(amountText, currency); err == nil {
			minor = &n
		}
	}
	account := webhookAccount(data)
	return &mb.ProviderWebhookEvent{
		ProviderReference: providerRef,
		Status:            flutterwaveStatus(stringValue(data["status"])),
		EventType:         configx.First(stringValue(payload["type"]), stringValue(payload["event"])),
		ExternalReference: stringValue(data["reference"]),
		Amount:            minor,
		Currency:          currency,
		Account:           account,
		Raw:               payload,
	}, nil
}

func webhookAccount(data map[string]any) string {
	if pm, ok := data["payment_method"].(map[string]any); ok {
		if mm, ok := pm["mobile_money"].(map[string]any); ok {
			phone := stringValue(mm["phone_number"])
			cc := strings.TrimPrefix(stringValue(mm["country_code"]), "+")
			if phone != "" {
				d := msisdn.Digits(phone)
				if cc != "" && !strings.HasPrefix(d, cc) {
					d = cc + strings.TrimLeft(d, "0")
				}
				return "+" + d
			}
		}
	}
	if recipient, ok := data["recipient"].(map[string]any); ok {
		if mm, ok := recipient["mobile_money"].(map[string]any); ok {
			if d := msisdn.Digits(stringValue(mm["msisdn"])); d != "" {
				return "+" + d
			}
		}
	}
	return ""
}

func flutterwaveStatus(v string) string {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "SUCCESS", "SUCCESSFUL", "SUCCEEDED", "COMPLETED":
		return mb.TxSucceeded
	case "PROCESSING", "IN_PROGRESS":
		return mb.TxProcessing
	case "PENDING", "NEW", "QUEUED":
		return mb.TxPending
	case "FAILED", "FAILURE", "ERROR":
		return mb.TxFailed
	case "CANCELLED", "CANCELED", "REVERSED":
		return mb.TxCancelled
	case "EXPIRED":
		return mb.TxExpired
	default:
		return mb.TxUnknown
	}
}

func flutterwaveReference(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	s = b.String()
	if len(s) > 42 {
		s = s[:42]
	}
	if len(s) < 6 {
		s += "-momobase"
		if len(s) > 42 {
			s = s[:42]
		}
	}
	return s
}

func amountNumber(minor int64, currency string) any {
	text := mb.FormatAmountMinor(minor, currency)
	if strings.Contains(text, ".") {
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			return f
		}
	}
	if n, err := strconv.ParseInt(text, 10, 64); err == nil {
		return n
	}
	return text
}

func numberValue(v any) string {
	switch n := v.(type) {
	case json.Number:
		return n.String()
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(n), 'f', -1, 64)
	case int:
		return strconv.Itoa(n)
	case int64:
		return strconv.FormatInt(n, 10)
	case string:
		return n
	default:
		return ""
	}
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	if n, ok := v.(json.Number); ok {
		return n.String()
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func splitName(name string) (first, last string) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func truncate(s string, n int) string {
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
var _ mb.HealthChecker = (*Provider)(nil)
var _ mb.WebhookVerifier = (*Provider)(nil)
var _ mb.RequestValidator = (*Provider)(nil)
