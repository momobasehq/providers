// Package mtn implements the MTN MoMo collections and disbursements APIs.
package mtn

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	mb "github.com/momobasehq/momobase/providers"
	"github.com/momobasehq/providers/internal/configx"
	"github.com/momobasehq/providers/internal/httpx"
	"github.com/momobasehq/providers/internal/msisdn"
	"github.com/momobasehq/providers/internal/textx"
	"github.com/momobasehq/providers/internal/token"
)

const sandboxURL = "https://sandbox.momodeveloper.mtn.com"

type credentials struct{ subscriptionKey, apiUser, apiKey string }
type state struct {
	baseURL, target          string
	collection, disbursement credentials
	caps                     []mb.Capability
}

// Provider is initialized once by the Momobase runtime before first use;
// Init is not safe to call concurrently with the payment methods.
type Provider struct {
	s                 state
	client            *http.Client
	collectionToken   token.Cache
	disbursementToken token.Cache
}

func New(*slog.Logger) mb.PaymentProvider { return &Provider{client: httpx.Client()} }

func (p *Provider) Init(_ context.Context, c mb.ProviderConfig) error {
	env := configx.Environment(c)
	base := mb.ConfigString(c, "base_url")
	target := mb.ConfigString(c, "target_environment")
	if base == "" {
		if env != "sandbox" {
			return fmt.Errorf("mtn: base_url is required outside sandbox")
		}
		base = sandboxURL
	}
	if target == "" {
		if env == "sandbox" {
			target = "sandbox"
		} else {
			return fmt.Errorf("mtn: target_environment is required outside sandbox")
		}
	}
	sharedSub := mb.ConfigString(c, "subscription_key")
	sharedUser := mb.ConfigString(c, "api_user")
	sharedKey := mb.ConfigString(c, "api_key")
	col := credentials{
		subscriptionKey: mb.First(mb.ConfigString(c, "collection_subscription_key"), sharedSub),
		apiUser:         mb.First(mb.ConfigString(c, "collection_api_user"), sharedUser),
		apiKey:          mb.First(mb.ConfigString(c, "collection_api_key"), sharedKey),
	}
	dis := credentials{
		subscriptionKey: mb.First(mb.ConfigString(c, "disbursement_subscription_key"), sharedSub),
		apiUser:         mb.First(mb.ConfigString(c, "disbursement_api_user"), sharedUser),
		apiKey:          mb.First(mb.ConfigString(c, "disbursement_api_key"), sharedKey),
	}
	var caps []mb.Capability
	if complete(col) {
		caps = append(caps, mb.Capability{ServiceType: mb.ServiceCollection, PaymentMethod: mb.PaymentMethodMomo})
	}
	if complete(dis) {
		caps = append(caps, mb.Capability{ServiceType: mb.ServiceDisbursement, PaymentMethod: mb.PaymentMethodMomo})
	}
	if len(caps) == 0 {
		return fmt.Errorf("mtn: configure collection_* and/or disbursement_* credentials")
	}
	p.s = state{baseURL: strings.TrimRight(base, "/"), target: target, collection: col, disbursement: dis, caps: caps}
	return nil
}

func complete(c credentials) bool {
	return c.subscriptionKey != "" && c.apiUser != "" && c.apiKey != ""
}
func (p *Provider) Capabilities() []mb.Capability { return p.s.caps }

func (p *Provider) ValidateRequest(_ context.Context, r *mb.PaymentRequest) error {
	if r.PaymentMethod != mb.PaymentMethodMomo {
		return fmt.Errorf("mtn: only mobile money is supported")
	}
	e, err := msisdn.E164(r.Account, r.Country)
	if err != nil {
		return err
	}
	r.Account = msisdn.Digits(e)
	r.Scheme = strings.ToUpper(strings.TrimSpace(r.Scheme))
	return nil
}

func (p *Provider) accessToken(ctx context.Context, product string, s state) (string, error) {
	var c credentials
	var cache *token.Cache
	switch product {
	case "collection":
		c = s.collection
		cache = &p.collectionToken
	case "disbursement":
		c = s.disbursement
		cache = &p.disbursementToken
	default:
		return "", fmt.Errorf("mtn: invalid product")
	}
	if !complete(c) {
		return "", fmt.Errorf("mtn: %s is not configured", product)
	}
	return cache.Get(ctx, func(ctx context.Context) (string, time.Duration, error) {
		var out struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}
		h := map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(c.apiUser+":"+c.apiKey)), "Ocp-Apim-Subscription-Key": c.subscriptionKey}
		if err := httpx.JSON(ctx, p.client, http.MethodPost, s.baseURL+"/"+product+"/token/", h, nil, &out); err != nil {
			return "", 0, err
		}
		if out.AccessToken == "" {
			return "", 0, fmt.Errorf("mtn: empty access token")
		}
		return out.AccessToken, time.Duration(out.ExpiresIn) * time.Second, nil
	})
}

func (p *Provider) headers(token string, c credentials, s state) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token, "Ocp-Apim-Subscription-Key": c.subscriptionKey, "X-Target-Environment": s.target}
}

func (p *Provider) Collect(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	s := p.s
	tok, err := p.accessToken(ctx, "collection", s)
	if err != nil {
		return nil, err
	}
	ref := uuid.NewString()
	body := map[string]any{"amount": mb.FormatAmountMinor(r.Amount, r.Currency), "currency": r.Currency, "externalId": r.TransactionID, "payer": map[string]string{"partyIdType": "MSISDN", "partyId": r.Account}, "payerMessage": textx.Trim(r.Description, 160), "payeeNote": textx.Trim(mb.First(r.Reference, r.Description), 160)}
	h := p.headers(tok, s.collection, s)
	h["X-Reference-Id"] = ref
	if err := httpx.JSON(ctx, p.client, http.MethodPost, s.baseURL+"/collection/v1_0/requesttopay", h, body, nil); err != nil {
		return nil, err
	}
	return &mb.ProviderPaymentResponse{ProviderReference: ref, Status: mb.TxPending, Message: "MTN request to pay accepted", Raw: map[string]any{"reference_id": ref}}, nil
}

func (p *Provider) Disburse(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	s := p.s
	tok, err := p.accessToken(ctx, "disbursement", s)
	if err != nil {
		return nil, err
	}
	ref := uuid.NewString()
	body := map[string]any{"amount": mb.FormatAmountMinor(r.Amount, r.Currency), "currency": r.Currency, "externalId": r.TransactionID, "payee": map[string]string{"partyIdType": "MSISDN", "partyId": r.Account}, "payerMessage": textx.Trim(r.Description, 160), "payeeNote": textx.Trim(mb.First(r.Reference, r.Description), 160)}
	h := p.headers(tok, s.disbursement, s)
	h["X-Reference-Id"] = ref
	if err := httpx.JSON(ctx, p.client, http.MethodPost, s.baseURL+"/disbursement/v1_0/transfer", h, body, nil); err != nil {
		return nil, err
	}
	return &mb.ProviderPaymentResponse{ProviderReference: ref, Status: mb.TxPending, Message: "MTN transfer accepted", Raw: map[string]any{"reference_id": ref}}, nil
}

type txStatus struct {
	Status                 string `json:"status"`
	Reason                 string `json:"reason"`
	FinancialTransactionID string `json:"financialTransactionId"`
	ExternalID             string `json:"externalId"`
}

func (p *Provider) QueryTransaction(ctx context.Context, ref, _ string) (*mb.ProviderTransactionStatus, error) {
	s := p.s
	products := []struct {
		name, path string
		c          credentials
	}{{"collection", "/collection/v1_0/requesttopay/", s.collection}, {"disbursement", "/disbursement/v1_0/transfer/", s.disbursement}}
	var errs []string
	for _, x := range products {
		if !complete(x.c) {
			continue
		}
		tok, err := p.accessToken(ctx, x.name, s)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		var out txStatus
		if err = httpx.JSON(ctx, p.client, http.MethodGet, s.baseURL+x.path+ref, p.headers(tok, x.c, s), nil, &out); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		return &mb.ProviderTransactionStatus{ProviderReference: ref, Status: status(out.Status), Message: mb.First(out.Reason, out.Status)}, nil
	}
	return nil, fmt.Errorf("mtn: transaction query failed: %s", strings.Join(errs, "; "))
}

func (p *Provider) HealthCheck(ctx context.Context) error {
	s := p.s
	if complete(s.collection) {
		if _, err := p.accessToken(ctx, "collection", s); err != nil {
			return err
		}
	}
	if complete(s.disbursement) {
		if _, err := p.accessToken(ctx, "disbursement", s); err != nil {
			return err
		}
	}
	return nil
}

func status(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "PENDING") {
		return mb.TxPending
	}
	return mb.PaymentStatus(v)
}

var _ mb.Collector = (*Provider)(nil)
var _ mb.Disburser = (*Provider)(nil)
var _ mb.TransactionQuerier = (*Provider)(nil)
var _ mb.HealthChecker = (*Provider)(nil)
var _ mb.RequestValidator = (*Provider)(nil)
