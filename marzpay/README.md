# MarzPay

Mobile money and card collections plus mobile money payouts. The mobile network is resolved by MarzPay from the phone number and country. Card collections return a hosted checkout `redirect_url` in the response's `Raw` map — send the payer there to complete payment.

📖 [MarzPay API documentation](https://wallet.wearemarz.com/documentation/api)

## Register

```go
import (
    "github.com/momobasehq/momobase"
    "github.com/momobasehq/providers/marzpay"
)

instance, err := momobase.New(
    momobase.WithProvider("marzpay", marzpay.New),
)
```

See the [repository README](../README.md#register-with-momobase) for how webhooks are routed.

## Configuration

`environment` is supplied by Momobase and is authoritative — do not set it yourself. It selects the sandbox or live defaults for any `base_url` you leave unset.

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
