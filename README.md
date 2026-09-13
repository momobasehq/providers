# Momobase providers

Provider adapters for [`github.com/momobasehq/momobase`](https://pkg.go.dev/github.com/momobasehq/momobase), following the [Momobase provider guide](https://momobase.dev/library/providers) and [provider API reference](https://momobase.dev/library/provider-api).

Each adapter lives in its own root package. Applications import only the packages they register; shared implementation details stay under `internal/`.

## Packages

| Package | Collections | Disbursements | Verified webhooks | Notes |
| --- | --- | --- | --- | --- |
| `mtn` | MoMo | MoMo | No | MTN MoMo Collection + Disbursement APIs |
| `airtel` | MoMo | MoMo* | No | Disbursement is enabled when `pin` or `encrypted_pin` is configured; optional Uganda request signing |
| `yopayments` | MoMo | MoMo | No | Yo! Payments XML API, non-blocking operations |
| `marzpay` | MoMo, Card | MoMo | Yes | Card collections return a hosted `redirect_url` in `Raw` |
| `flutterwave` | MoMo | MoMo | Yes | Flutterwave v4 OAuth, mobile-money charges and direct transfers |

\* Airtel collection is always exposed once credentials are valid. Disbursement requires a configured PIN.

## Install only what you use

```bash
go get github.com/momobasehq/providers/mtn
go get github.com/momobasehq/providers/airtel
```

You can install/import any other provider package the same way. Go resolves the module once, but your application only compiles the packages you import.

## Register with Momobase

```go
package main

import (
    "github.com/momobasehq/momobase"
    "github.com/momobasehq/providers/airtel"
    "github.com/momobasehq/providers/marzpay"
    "github.com/momobasehq/providers/mtn"
)

func main() {
    instance, err := momobase.New(
        momobase.WithProvider("mtn", mtn.New),
        momobase.WithProvider("airtel", airtel.New),
        momobase.WithProvider("marzpay", marzpay.New),
    )
    if err != nil {
        panic(err)
    }
    defer instance.Close()
}
```

Provider account configuration is passed by Momobase to each adapter's `Init`. `environment` is supplied by Momobase and is authoritative.

## Configuration

### MTN

Use either shared credentials or product-specific credentials.

```text
subscription_key                 shared fallback
api_user                         shared fallback
api_key                          shared fallback
collection_subscription_key     optional product override
collection_api_user             optional product override
collection_api_key              optional product override
disbursement_subscription_key   optional product override
disbursement_api_user           optional product override
disbursement_api_key            optional product override
target_environment              defaults to sandbox only
base_url                         defaults to MTN sandbox only
```

At least one complete collection/disbursement credential set is required. Production intentionally requires explicit `base_url` and `target_environment` so a country-specific live endpoint is not guessed.

### Airtel Money

```text
client_id          required
client_secret      required
country            required, ISO alpha-2 (for example UG)
currency           required (for example UGX)
pin                enables disbursement; encrypted with Airtel RSA key
encrypted_pin      alternative to pin when already encrypted
sign_requests      optional bool; enables x-signature/x-key request signing
public_key         optional PEM/base64 RSA public key; otherwise fetched when needed
base_url            optional; defaults to openapiuat.airtel.africa or openapi.airtel.africa
```

`sign_requests` exists for Airtel deployments that require encrypted request signing. Use `base_url` when your Airtel market/account is provisioned on a market-specific hostname.

### Yo! Payments

```text
username    required API username
password    required API password
base_url    optional; defaults by environment
```

Sandbox: `https://sandbox.yo.co.ug/services/yopaymentsdev/task.php`  
Live: `https://paymentsapi1.yo.co.ug/ybs/task.php`

### MarzPay

```text
api_key                  required
api_secret               required
callback_url             optional
webhook_signing_secret   required when callback_url is configured
base_url                  optional; defaults to https://wallet.wearemarz.com/api/v1
```

Mobile-money providers are detected by MarzPay from the E.164 phone number and `country`. Card collection uses `method=card` and exposes MarzPay's `redirect_url` through the response `Raw` map.

### Flutterwave

```text
client_id         required v4 OAuth client ID
client_secret     required v4 OAuth client secret
webhook_secret    required dashboard webhook secret hash
redirect_url      optional collection redirect URL
callback_url      optional per-transfer callback URL
base_url           optional; defaults by environment
```

Sandbox: `https://developersandbox-api.flutterwave.com`  
Live: `https://f4bexperience.flutterwave.com`

For mobile money, Momobase `Scheme` must name the network (for example `MTN` or `AIRTEL`). Collections also require `Email`, because Flutterwave v4 requires a customer object before a mobile-money payment method can be created.

## Webhook policy

A Momobase `WebhookVerifier` is implemented only when the upstream exposes cryptographic verification:

- MarzPay: HMAC-SHA256 over `{timestamp}.{raw_body}` using `X-MarzPay-Timestamp` and `X-MarzPay-Signature`.
- Flutterwave: HMAC-SHA256 of the raw body, base64 encoded, compared to `flutterwave-signature`.

MTN, Airtel, and Yo! Payments are deliberately polling/reconciliation-first here rather than accepting an unauthenticated callback as authoritative.

## Shared internals

```text
internal/airtelcrypto   Airtel AES/RSA request signing and PIN encryption
internal/configx        ProviderConfig helpers
internal/httpx          Context-bound HTTP, JSON/form/multipart helpers and redacted errors
internal/msisdn         E.164/local phone normalization for supported African markets
internal/token          Concurrent OAuth/access-token cache
internal/uuidx          UUID v4 generation
```

## Provider API invariants

The adapters follow Momobase v0.3.0's rules: each declared payment capability has its operation interface and `TransactionQuerier`; mutable configuration/token state is concurrency-safe; `RequestValidator` only rewrites `Account`/`Scheme`; amounts remain integer minor units inside Momobase; HTTP requests honor caller context; provider statuses are normalized to Momobase transaction states.

## Upstream references

- Momobase: https://momobase.dev/library/providers and https://momobase.dev/library/provider-api
- MTN MoMo: https://momodeveloper.mtn.com/
- Airtel Africa Developer Portal: https://developers.airtel.africa/
- Yo! Payments: https://paymentsweb.yo.co.ug/index.php and its public sandbox/developer material
- MarzPay: https://wallet.wearemarz.com/documentation/api, `/documentation/collections`, `/documentation/send-money`, `/documentation/webhooks`
- Flutterwave v4: https://developer.flutterwave.com/docs/mobile-money, `/docs/mobile-money-1`, `/docs/authentication`, `/docs/webhooks`

## Verification before release

Run against the real dependency and sandboxes before tagging:

```bash
go mod tidy
go test -race ./...
go vet ./...
```

Then create sandbox provider accounts in Momobase and exercise collection/disbursement, reconciliation, duplicate requests, timeout/cancellation, and signed webhook paths before enabling live money movement.
