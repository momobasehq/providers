# MTN MoMo

Collections through request-to-pay and disbursements through transfer, on the MTN MoMo Collection and Disbursement APIs. Capabilities are derived from the credentials you provide: supply a complete collection set, a complete disbursement set, or both.

📖 [MTN MoMo Developer Portal](https://momodeveloper.mtn.com/)

## Register

```go
import (
    "github.com/momobasehq/momobase"
    "github.com/momobasehq/providers/mtn"
)

instance, err := momobase.New(
    momobase.WithProvider("mtn", mtn.New),
)
```

See the [repository README](../README.md#register-with-momobase) for how webhooks are routed.

## Configuration

`environment` is supplied by Momobase and is authoritative — do not set it yourself. It selects the sandbox or live defaults for any `base_url` you leave unset.

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
