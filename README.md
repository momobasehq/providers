# Momobase providers

Payment provider adapters for [Momobase](https://github.com/momobasehq/momobase), covering mobile money, cards, and bank rails across African markets.

Each adapter is its own package, so your application compiles only the providers it registers.

| Package | Collections | Disbursements | Verified webhooks |
| --- | --- | --- | --- |
| [`mtn`](mtn/) | Mobile money | Mobile money | No |
| [`airtel`](airtel/) | Mobile money | Mobile money | No |
| [`yopayments`](yopayments/) | Mobile money | Mobile money | No |
| [`marzpay`](marzpay/) | Mobile money, card | Mobile money | Yes |
| [`flutterwave`](flutterwave/) | Mobile money | Mobile money | Yes |

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

> [!IMPORTANT]
> The name a provider is registered under — `"mtn"` above — selects which adapter a
> provider account uses. It is **not** the webhook path.
>
> Momobase receives webhooks at `POST /webhooks/{providerAccountID}`, using the ID of
> the provider account you create in Momobase:
>
> ```
> https://momobase.local/webhooks/pacc_5f8d2c1e-9b3a-4d7e-8c6f-1a2b3c4d5e6f
> ```
>
> Incoming requests must also carry an `X-Webhook-Secret` header matching that account's
> `webhook_secret`, on top of the provider's own signature verification. Configure this
> exact URL at the provider — a guessed path such as `/webhooks/mtn` will not resolve.

## Configuration

Each provider documents its own configuration keys, an example configuration, and its registration snippet:

- [`mtn`](mtn/README.md) — MTN MoMo
- [`airtel`](airtel/README.md) — Airtel Money
- [`yopayments`](yopayments/README.md) — Yo! Payments
- [`marzpay`](marzpay/README.md) — MarzPay
- [`flutterwave`](flutterwave/README.md) — Flutterwave
