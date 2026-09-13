# Momobase providers

Payment provider adapters for [Momobase](https://github.com/momobasehq/momobase), covering mobile money, cards, and bank rails across African markets.

Each adapter is its own package, so your application compiles only the providers it registers.

| Package | Collections | Disbursements | Verified webhooks |
| --- | --- | --- | --- |
| [`mtn`](#mtn-momo) | Mobile money | Mobile money | No |
| [`airtel`](#airtel-money) | Mobile money | Mobile money | No |
| [`yopayments`](#yo-payments) | Mobile money | Mobile money | No |
| [`marzpay`](#marzpay) | Mobile money, card | Mobile money | Yes |
| [`flutterwave`](#flutterwave) | Mobile money | Mobile money | Yes |

## Install

```bash
go get github.com/momobasehq/providers/mtn
```

Install any other provider the same way — Go resolves the module once, and only the packages you import are compiled.

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

## Configuration

Each provider account's configuration is stored in Momobase and passed to the adapter when it starts. The JSON below is what goes into that configuration.

`environment` is supplied by Momobase and is authoritative — do not set it yourself. It selects the sandbox or live defaults for any `base_url` you leave unset.

---

### MTN MoMo

Collections through request-to-pay and disbursements through transfer, on the MTN MoMo Collection and Disbursement APIs. Capabilities are derived from the credentials you provide: supply a complete collection set, a complete disbursement set, or both.

📖 [MTN MoMo Developer Portal](https://momodeveloper.mtn.com/)

| Key | Required | Description |
| --- | --- | --- |
| `subscription_key` | Yes¹ | Subscription key used when no product-specific key is set |
| `api_user` | Yes¹ | API user ID |
| `api_key` | Yes¹ | API key issued for the API user |
| `collection_subscription_key` | No | Overrides `subscription_key` for collections |
| `collection_api_user` | No | Overrides `api_user` for collections |
| `collection_api_key` | No | Overrides `api_key` for collections |
| `disbursement_subscription_key` | No | Overrides `subscription_key` for disbursements |
| `disbursement_api_user` | No | Overrides `api_user` for disbursements |
| `disbursement_api_key` | No | Overrides `api_key` for disbursements |
| `target_environment` | Outside sandbox | Target environment for your market |
| `base_url` | Outside sandbox | Your market's API host |

¹ Either the shared credentials or the matching product-specific overrides. A product is enabled only when its subscription key, API user, and API key are all present.

Outside sandbox, `target_environment` and `base_url` must be set explicitly so a live market host is never guessed.

```json
{
  "subscription_key": "your-subscription-key",
  "api_user": "your-api-user",
  "api_key": "your-api-key",
  "target_environment": "sandbox",
  "base_url": "https://sandbox.momodeveloper.mtn.com"
}
```

Separate credentials per product:

```json
{
  "collection_subscription_key": "your-collection-subscription-key",
  "collection_api_user": "your-collection-api-user",
  "collection_api_key": "your-collection-api-key",
  "disbursement_subscription_key": "your-disbursement-subscription-key",
  "disbursement_api_user": "your-disbursement-api-user",
  "disbursement_api_key": "your-disbursement-api-key",
  "target_environment": "sandbox",
  "base_url": "https://sandbox.momodeveloper.mtn.com"
}
```

---

### Airtel Money

Airtel Money collections and disbursements across Airtel Africa markets. One account serves a single country and currency. Disbursement is enabled only when a PIN is configured; collection is always available.

📖 [Airtel Africa Developer Portal](https://developers.airtel.africa/)

| Key | Required | Description |
| --- | --- | --- |
| `client_id` | Yes | OAuth client ID |
| `client_secret` | Yes | OAuth client secret |
| `country` | Yes | ISO 3166-1 alpha-2 country code, for example `UG` |
| `currency` | Yes | ISO 4217 currency code, for example `UGX` |
| `pin` | No | Enables disbursement; encrypted with Airtel's RSA key before use |
| `encrypted_pin` | No | Already-encrypted alternative to `pin` |
| `public_key` | No | PEM or base64 RSA public key; fetched from Airtel when omitted |
| `sign_requests` | No | Set `true` for deployments that require encrypted request signing |
| `base_url` | No | Market-specific host; defaults by environment |

Requests are rejected when their country or currency does not match the account's.

```json
{
  "client_id": "your-client-id",
  "client_secret": "your-client-secret",
  "country": "UG",
  "currency": "UGX",
  "pin": "1234",
  "sign_requests": false
}
```

---

### Yo! Payments

Mobile money collections and withdrawals on the Yo! Payments API. Operations are submitted as non-blocking requests and resolved by querying the transaction.

📖 [Yo! Payments](https://paymentsweb.yo.co.ug/index.php)

| Key | Required | Description |
| --- | --- | --- |
| `username` | Yes | API username |
| `password` | Yes | API password |
| `base_url` | No | API endpoint; defaults by environment |

```json
{
  "username": "your-api-username",
  "password": "your-api-password",
  "base_url": "yo-payments-base-url"
}
```

---

### MarzPay

Mobile money and card collections plus mobile money payouts. The mobile network is resolved by MarzPay from the phone number and country. Card collections return a hosted checkout `redirect_url` in the response's `Raw` map — send the payer there to complete payment.

📖 [MarzPay API documentation](https://wallet.wearemarz.com/documentation/api)

| Key | Required | Description |
| --- | --- | --- |
| `api_key` | Yes | API key |
| `api_secret` | Yes | API secret |
| `callback_url` | No | Webhook destination for transaction updates |
| `webhook_signing_secret` | With `callback_url` | Secret used to verify webhook signatures |
| `base_url` | No | API endpoint; defaults to MarzPay's live API |

Configuring `callback_url` without `webhook_signing_secret` is rejected at startup, so callbacks are never accepted unverified.

```json
{
  "api_key": "your-api-key",
  "api_secret": "your-api-secret",
  "callback_url": "https://momobase.local/webhooks/marzpay",
  "webhook_signing_secret": "your-webhook-signing-secret"
}
```

---

### Flutterwave

Mobile money collections and payouts on Flutterwave v4. Collections require the payer's `Email`, because v4 creates a customer before a mobile money payment method. `Scheme` must name the mobile network, for example `MTN` or `AIRTEL`.

📖 [Flutterwave mobile money documentation](https://developer.flutterwave.com/docs/mobile-money)

| Key | Required | Description |
| --- | --- | --- |
| `client_id` | Yes | OAuth client ID |
| `client_secret` | Yes | OAuth client secret |
| `webhook_secret` | Yes | Webhook secret hash from your dashboard |
| `redirect_url` | No | Where payers return after a collection |
| `callback_url` | No | Per-transfer callback destination |
| `base_url` | No | API endpoint; defaults by environment |

```json
{
  "client_id": "your-client-id",
  "client_secret": "your-client-secret",
  "webhook_secret": "your-webhook-secret-hash",
  "callback_url": "https://momobase.local/webhooks/flutterwave"
}
```
