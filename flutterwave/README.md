# Flutterwave

Mobile money collections and payouts on Flutterwave v4. Collections require the payer's `Email`, because v4 creates a customer before a mobile money payment method. `Scheme` must name the mobile network, for example `MTN` or `AIRTEL`.

📖 [Flutterwave mobile money documentation](https://developer.flutterwave.com/docs/mobile-money)

## Register

```go
import (
    "github.com/momobasehq/momobase"
    "github.com/momobasehq/providers/flutterwave"
)

instance, err := momobase.New(
    momobase.WithProvider("flutterwave", flutterwave.New),
)
```

See the [repository README](../README.md#register-with-momobase) for how webhooks are routed.

## Configuration

`environment` is supplied by Momobase and is authoritative — do not set it yourself. It selects the sandbox or live defaults for any `base_url` you leave unset.

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
