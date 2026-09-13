// Package yopayments implements the Yo! Payments XML API.
package yopayments

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	mb "github.com/momobasehq/momobase/providers"
	"github.com/momobasehq/providers/internal/configx"
	"github.com/momobasehq/providers/internal/httpx"
	"github.com/momobasehq/providers/internal/msisdn"
)

const (
	sandboxURL = "https://sandbox.yo.co.ug/services/yopaymentsdev/task.php"
	liveURL    = "https://paymentsapi1.yo.co.ug/ybs/task.php"
)

type state struct {
	endpoint, username, password string
	caps                         []mb.Capability
}

// Provider is initialized once by the Momobase runtime before first use;
// Init is not safe to call concurrently with the payment methods.
type Provider struct {
	s      state
	client *http.Client
}

func New(*slog.Logger) mb.PaymentProvider { return &Provider{client: httpx.Client()} }
func (p *Provider) Init(_ context.Context, c mb.ProviderConfig) error {
	if err := configx.Require(c, "username", "password"); err != nil {
		return fmt.Errorf("yopayments: %w", err)
	}
	endpoint := mb.ConfigString(c, "base_url")
	if endpoint == "" {
		if e := configx.Environment(c); e == "production" || e == "live" {
			endpoint = liveURL
		} else {
			endpoint = sandboxURL
		}
	}
	s := state{endpoint: endpoint, username: mb.ConfigString(c, "username"), password: mb.ConfigString(c, "password"), caps: []mb.Capability{{ServiceType: mb.ServiceCollection, PaymentMethod: mb.PaymentMethodMomo}, {ServiceType: mb.ServiceDisbursement, PaymentMethod: mb.PaymentMethodMomo}}}
	p.s = s
	return nil
}
func (p *Provider) Capabilities() []mb.Capability { return p.s.caps }
func (p *Provider) ValidateRequest(_ context.Context, r *mb.PaymentRequest) error {
	if r.PaymentMethod != mb.PaymentMethodMomo {
		return fmt.Errorf("yopayments: only mobile money is supported")
	}
	e, err := msisdn.E164(r.Account, r.Country)
	if err != nil {
		return err
	}
	r.Account = msisdn.Digits(e)
	return nil
}

type autoCreate struct {
	XMLName xml.Name  `xml:"AutoCreate"`
	Request yoRequest `xml:"Request"`
}
type yoRequest struct {
	APIUsername                 string `xml:"APIUsername"`
	APIPassword                 string `xml:"APIPassword"`
	Method                      string `xml:"Method"`
	NonBlocking                 string `xml:"NonBlocking,omitempty"`
	Account                     string `xml:"Account,omitempty"`
	Amount                      string `xml:"Amount,omitempty"`
	Narrative                   string `xml:"Narrative,omitempty"`
	ExternalReference           string `xml:"ExternalReference,omitempty"`
	TransactionReference        string `xml:"TransactionReference,omitempty"`
	PrivateTransactionReference string `xml:"PrivateTransactionReference,omitempty"`
}
type yoEnvelope struct {
	Response yoResponse `xml:"Response"`
}
type yoResponse struct {
	Status                    string `xml:"Status"`
	StatusCode                string `xml:"StatusCode"`
	StatusMessage             string `xml:"StatusMessage"`
	TransactionStatus         string `xml:"TransactionStatus"`
	ErrorMessageCode          string `xml:"ErrorMessageCode"`
	ErrorMessage              string `xml:"ErrorMessage"`
	TransactionReference      string `xml:"TransactionReference"`
	MNOTransactionReferenceID string `xml:"MNOTransactionReferenceId"`
	IssuedReceiptNumber       string `xml:"IssuedReceiptNumber"`
	Amount                    string `xml:"Amount"`
	CurrencyCode              string `xml:"CurrencyCode"`
}

func (p *Provider) call(ctx context.Context, s state, r yoRequest) (yoResponse, error) {
	body, err := xml.Marshal(autoCreate{Request: r})
	if err != nil {
		return yoResponse{}, err
	}
	body = append([]byte(xml.Header), body...)
	raw, err := httpx.Do(ctx, p.client, http.MethodPost, s.endpoint, nil, "text/xml; charset=utf-8", body)
	if err != nil {
		return yoResponse{}, err
	}
	var out yoEnvelope
	if err = xml.Unmarshal(raw, &out); err != nil {
		return yoResponse{}, fmt.Errorf("yopayments: decode response: %w", err)
	}
	if out.Response.Status == "" && out.Response.TransactionStatus == "" {
		return yoResponse{}, fmt.Errorf("yopayments: empty response")
	}
	return out.Response, nil
}
func baseRequest(s state, method string) yoRequest {
	return yoRequest{APIUsername: s.username, APIPassword: s.password, Method: method}
}
func (p *Provider) pay(ctx context.Context, r mb.PaymentRequest, method string) (*mb.ProviderPaymentResponse, error) {
	s := p.s
	q := baseRequest(s, method)
	q.NonBlocking = "TRUE"
	q.Account = r.Account
	q.Amount = mb.FormatAmountMinor(r.Amount, r.Currency)
	q.Narrative = mb.First(r.Description, r.Reference, "Momobase payment")
	q.ExternalReference = mb.First(r.TransactionID, r.Reference)
	out, err := p.call(ctx, s, q)
	if err != nil {
		return nil, err
	}
	if out.TransactionReference == "" && strings.EqualFold(out.Status, "ERROR") {
		return nil, fmt.Errorf("yopayments: %s", mb.First(out.ErrorMessage, out.StatusMessage, out.Status))
	}
	return &mb.ProviderPaymentResponse{ProviderReference: out.TransactionReference, Status: yoStatus(out.TransactionStatus, out.Status), Message: mb.First(out.ErrorMessage, out.StatusMessage, out.TransactionStatus, out.Status), Raw: rawMap(out)}, nil
}
func (p *Provider) Collect(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	return p.pay(ctx, r, "acdepositfunds")
}
func (p *Provider) Disburse(ctx context.Context, r mb.PaymentRequest) (*mb.ProviderPaymentResponse, error) {
	return p.pay(ctx, r, "acwithdrawfunds")
}
func (p *Provider) QueryTransaction(ctx context.Context, ref, _ string) (*mb.ProviderTransactionStatus, error) {
	s := p.s
	q := baseRequest(s, "actransactioncheckstatus")
	q.TransactionReference = ref
	out, err := p.call(ctx, s, q)
	if err != nil {
		return nil, err
	}
	return &mb.ProviderTransactionStatus{ProviderReference: mb.First(out.TransactionReference, ref), Status: yoStatus(out.TransactionStatus, out.Status), Message: mb.First(out.ErrorMessage, out.StatusMessage, out.TransactionStatus, out.Status)}, nil
}
func yoStatus(tx, top string) string {
	v := strings.ToUpper(mb.First(tx, top))
	switch v {
	case "OK":
		return mb.TxSucceeded
	case "PENDING":
		return mb.TxPending
	case "INDETERMINATE":
		return mb.TxProcessing
	case "FAIL", "ERROR":
		return mb.TxFailed
	default:
		return mb.PaymentStatus(v)
	}
}
func rawMap(r yoResponse) map[string]any {
	return map[string]any{"status": r.Status, "status_code": r.StatusCode, "status_message": r.StatusMessage, "transaction_status": r.TransactionStatus, "transaction_reference": r.TransactionReference, "mno_transaction_reference_id": r.MNOTransactionReferenceID, "issued_receipt_number": r.IssuedReceiptNumber, "error_message_code": r.ErrorMessageCode, "error_message": r.ErrorMessage, "amount": r.Amount, "currency_code": r.CurrencyCode}
}

var _ mb.Collector = (*Provider)(nil)
var _ mb.Disburser = (*Provider)(nil)
var _ mb.TransactionQuerier = (*Provider)(nil)
var _ mb.RequestValidator = (*Provider)(nil)
