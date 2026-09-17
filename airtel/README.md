# Airtel Money

Airtel Money collections and disbursements across Airtel Africa markets. One account serves a single country and currency. Disbursement is enabled only when a PIN is configured; collection is always available.

📖 [Airtel Africa Developer Portal](https://developers.airtel.africa/)

## Register

```go
import (
    "github.com/momobasehq/momobase"
    "github.com/momobasehq/providers/airtel"
)

instance, err := momobase.New(
    momobase.WithProvider("airtel", airtel.New),
)
```

See the [repository README](../README.md#register-with-momobase) for how webhooks are routed.

## Configuration

`environment` is supplied by Momobase and is authoritative — do not set it yourself. It selects the sandbox or live defaults for any `base_url` you leave unset.

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
