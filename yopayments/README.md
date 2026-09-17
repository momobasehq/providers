# Yo! Payments

Mobile money collections and withdrawals on the Yo! Payments API. Operations are submitted as non-blocking requests and resolved by querying the transaction.

📖 [Yo! Payments](https://paymentsweb.yo.co.ug/index.php)

## Register

```go
import (
    "github.com/momobasehq/momobase"
    "github.com/momobasehq/providers/yopayments"
)

instance, err := momobase.New(
    momobase.WithProvider("yopayments", yopayments.New),
)
```

See the [repository README](../README.md#register-with-momobase) for how webhooks are routed.

## Configuration

`environment` is supplied by Momobase and is authoritative — do not set it yourself. It selects the sandbox or live defaults for any `base_url` you leave unset.

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
