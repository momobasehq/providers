// Package airtel implements Airtel Money collections and disbursements.
package airtel

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	mb "github.com/momobasehq/momobase/providers"
	"github.com/momobasehq/providers/internal/airtelcrypto"
	"github.com/momobasehq/providers/internal/configx"
	"github.com/momobasehq/providers/internal/httpx"
	"github.com/momobasehq/providers/internal/msisdn"
	"github.com/momobasehq/providers/internal/token"
)

const (
	sandboxURL = "https://openapiuat.airtel.africa"
	liveURL    = "https://openapi.airtel.africa"
)

type state struct {
	baseURL, clientID, clientSecret, country, currency, pin, encryptedPIN, publicKey string
	sign                                                                             bool
	caps                                                                             []mb.Capability
}
type Provider struct {
	mu     sync.RWMutex
	s      state
	client *http.Client
	token  token.Cache
	keyMu  sync.Mutex
	key    *rsa.PublicKey
}

func New(*slog.Logger) mb.PaymentProvider { return &Provider{client: httpx.Client()} }

func (p *Provider) Init(_ context.Context, c mb.ProviderConfig) error {
	if err := configx.Require(c, "client_id", "client_secret", "country", "currency"); err != nil {
		return fmt.Errorf("airtel: %w", err)
	}
	env := configx.Environment(c)
	base := configx.String(c, "base_url")
	if base == "" {
		if env == "production" || env == "live" {
			base = liveURL
		} else {
			base = sandboxURL
		}
	}
	s := state{baseURL: strings.TrimRight(base, "/"), clientID: configx.String(c, "client_id"), clientSecret: configx.String(c, "client_secret"), country: strings.ToUpper(configx.String(c, "country")), currency: strings.ToUpper(configx.String(c, "currency")), pin: configx.String(c, "pin"), encryptedPIN: configx.String(c, "encrypted_pin"), publicKey: configx.String(c, "public_key"), sign: mb.ConfigBool(c, "sign_requests")}
	s.caps = []mb.Capability{{ServiceType: mb.ServiceCollection, PaymentMethod: mb.PaymentMethodMomo}}
	if s.pin != "" || s.encryptedPIN != "" {
		s.caps = append(s.caps, mb.Capability{ServiceType: mb.ServiceDisbursement, PaymentMethod: mb.PaymentMethodMomo})
	}
	p.mu.Lock()
	p.s = s
	p.mu.Unlock()
	p.token.Reset()
	p.keyMu.Lock()
	p.key = nil
	p.keyMu.Unlock()
	return nil
}
func (p *Provider) snapshot() state { p.mu.RLock(); defer p.mu.RUnlock(); return p.s }
func (p *Provider) Capabilities() []mb.Capability {
	s := p.snapshot()
	return append([]mb.Capability(nil), s.caps...)
}

func (p *Provider) ValidateRequest(_ context.Context, r *mb.PaymentRequest) error {
	s := p.snapshot()
	if r.PaymentMethod != mb.PaymentMethodMomo {
		return fmt.Errorf("airtel: only mobile money is supported")
	}
	if r.Country != "" && !strings.EqualFold(r.Country, s.country) {
		return fmt.Errorf("airtel: configured country is %s", s.country)
	}
	if r.Currency != "" && !strings.EqualFold(r.Currency, s.currency) {
		return fmt.Errorf("airtel: configured currency is %s", s.currency)
	}
	local, err := msisdn.Local(r.Account, s.country)
	if err != nil {
		return err
	}
	r.Account = local
	r.Scheme = "AIRTEL"
	return nil
}

func (p *Provider) accessToken(ctx context.Context, s state) (string, error) {
	return p.token.Get(ctx, func(ctx context.Context) (string, time.Duration, error) {
		var out struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}
		body := map[string]string{"client_id": s.clientID, "client_secret": s.clientSecret, "grant_type": "client_credentials"}
		if err := httpx.JSON(ctx, p.client, http.MethodPost, s.baseURL+"/auth/oauth2/token", nil, body, &out); err != nil {
			return "", 0, err
		}
		if out.AccessToken == "" {
			return "", 0, fmt.Errorf("airtel: empty access token")
		}
		return out.AccessToken, time.Duration(out.ExpiresIn) * time.Second, nil
	})
}
func headers(tok string, s state) map[string]string {
	return map[string]string{"Authorization": "Bearer " + tok, "X-Country": s.country, "X-Currency": s.currency, "Accept": "application/json"}
}

func (p *Provider) publicKey(ctx context.Context, tok string, s state) (*rsa.PublicKey, error) {
	p.keyMu.Lock()
	defer p.keyMu.Unlock()
	if p.key != nil {
		return p.key, nil
	}
	raw := s.publicKey
	if raw == "" {
		var out struct {
			Key  string `json:"key"`
			Data struct {
				Key string `json:"key"`
			} `json:"data"`
		}
		if err := httpx.JSON(ctx, p.client, http.MethodGet, s.baseURL+"/v1/rsa/encryption-keys", headers(tok, s), nil, &out); err != nil {
			return nil, fmt.Errorf("airtel encryption key: %w", err)
		}
		raw = configx.First(out.Key, out.Data.Key)
	}
	k, err := airtelcrypto.PublicKey(raw)
	if err != nil {
		return nil, err
	}
	p.key = k
	return k, nil
}

func (p *Provider) post(ctx context.Context, endpoint string, tok string, s state, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	h := headers(tok, s)
	if s.sign {
		k, err := p.publicKey(ctx, tok, s)
		if err != nil {
			return err
		}
		sig, key, err := airtelcrypto.Sign(k, b)
		if err != nil {
			return err
		}
		h["x-signature"] = sig
		h["x-key"] = key
	}
	raw, err := httpx.Do(ctx, p.client, http.MethodPost, endpoint, h, "application/json", b)
	if err != nil {
		return err
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

type envelope struct {
	Data struct {
		Transaction struct {
			ID            string `json:"id"`
			AirtelMoneyID string `json:"airtel_money_id"`
			Status        string `json:"status"`
			Message       string `json:"message"`
		} `json:"transaction"`
	} `json:"data"`
	Status struct {
		Success    bool   `json:"success"`
		Message    string `json:"message"`
		ResultCode string `json:"result_code"`
	} `json:"status"`
}

func amount(r mb.PaymentRequest) (float64, error) {
	return strconv.ParseFloat(mb.FormatAmountMinor(r.Amount, r.Currency), 64)
}

func (p *Provider) Collect(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	s := p.snapshot()
	tok, err := p.accessToken(ctx, s)
	if err != nil {
		return nil, err
	}
	a, err := amount(r)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"reference": configx.First(r.Reference, r.TransactionID), "subscriber": map[string]string{"country": s.country, "currency": s.currency, "msisdn": r.Account}, "transaction": map[string]any{"amount": a, "country": s.country, "currency": s.currency, "id": r.TransactionID}}
	var out envelope
	if err = p.post(ctx, s.baseURL+"/merchant/v2/payments/", tok, s, body, &out); err != nil {
		return nil, err
	}
	ref := configx.First(out.Data.Transaction.ID, r.TransactionID)
	msg := configx.First(out.Data.Transaction.Message, out.Status.Message, "Payment accepted")
	return &mb.ProviderPaymentResponse{ProviderReference: ref, Status: airtelStatus(out.Data.Transaction.Status), Message: msg, Raw: httpx.Map(out)}, nil
}

func (p *Provider) Disburse(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	s := p.snapshot()
	if s.pin == "" && s.encryptedPIN == "" {
		return nil, fmt.Errorf("airtel: disbursement is not configured")
	}
	tok, err := p.accessToken(ctx, s)
	if err != nil {
		return nil, err
	}
	a, err := amount(r)
	if err != nil {
		return nil, err
	}
	pin := s.encryptedPIN
	if pin == "" {
		k, err := p.publicKey(ctx, tok, s)
		if err != nil {
			return nil, err
		}
		pin, err = airtelcrypto.EncryptPIN(k, s.pin)
		if err != nil {
			return nil, err
		}
	}
	body := map[string]any{"payee": map[string]string{"msisdn": r.Account, "currency": s.currency}, "reference": configx.First(r.Reference, r.TransactionID), "pin": pin, "transaction": map[string]any{"amount": a, "currency": s.currency, "id": r.TransactionID, "type": "B2C"}}
	var out envelope
	if err = p.post(ctx, s.baseURL+"/standard/v1/disbursements/", tok, s, body, &out); err != nil {
		return nil, err
	}
	ref := configx.First(out.Data.Transaction.ID, r.TransactionID)
	msg := configx.First(out.Data.Transaction.Message, out.Status.Message, "Disbursement accepted")
	return &mb.ProviderPaymentResponse{ProviderReference: ref, Status: airtelStatus(out.Data.Transaction.Status), Message: msg, Raw: httpx.Map(out)}, nil
}

func (p *Provider) QueryTransaction(ctx context.Context, ref, _ string) (*mb.ProviderTransactionStatus, error) {
	s := p.snapshot()
	tok, err := p.accessToken(ctx, s)
	if err != nil {
		return nil, err
	}
	h := headers(tok, s)
	paths := []string{"/standard/v1/payments/"}
	if s.pin != "" || s.encryptedPIN != "" {
		paths = append(paths, "/standard/v1/disbursements/")
	}
	var errs []string
	for _, path := range paths {
		var out envelope
		if err = httpx.JSON(ctx, p.client, http.MethodGet, s.baseURL+path+ref, h, nil, &out); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		t := out.Data.Transaction
		return &mb.ProviderTransactionStatus{ProviderReference: configx.First(t.ID, ref), Status: airtelStatus(t.Status), Message: configx.First(t.Message, out.Status.Message, t.Status)}, nil
	}
	return nil, fmt.Errorf("airtel: transaction query failed: %s", strings.Join(errs, "; "))
}
func (p *Provider) HealthCheck(ctx context.Context) error {
	s := p.snapshot()
	_, err := p.accessToken(ctx, s)
	return err
}
func airtelStatus(v string) string {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "TS", "SUCCESS", "SUCCESSFUL":
		return mb.TxSucceeded
	case "DP", "PENDING":
		return mb.TxPending
	case "TIP", "PROCESSING":
		return mb.TxProcessing
	case "TF", "FAILED":
		return mb.TxFailed
	default:
		return mb.PaymentStatus(v)
	}
}

var _ mb.PaymentProvider = (*Provider)(nil)
var _ mb.Collector = (*Provider)(nil)
var _ mb.Disburser = (*Provider)(nil)
var _ mb.TransactionQuerier = (*Provider)(nil)
var _ mb.HealthChecker = (*Provider)(nil)
var _ mb.RequestValidator = (*Provider)(nil)
