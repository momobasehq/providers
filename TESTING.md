# Call for testers

These adapters are written against each provider's published API, but most of them have never run against a real merchant account in a real market. Unit tests cannot reach the parts that matter: live credentials, per-market number formats, the exact status strings a provider returns, and signed webhooks.

If you hold an account with any provider below, testing it and reporting what you see is the most useful contribution you can make. Every report feeds directly back into the adapters.

[Open a provider report →](../../issues/new?template=provider-report.yml)

## Before you start

> [!WARNING]
> **MarzPay has no sandbox endpoint.** The adapter always calls the live API
> (`https://wallet.wearemarz.com/api/v1`); `environment` does not change this.
> Whether money actually moves depends on your MarzPay account being in sandbox
> mode, which the adapter cannot check for you. Confirm your account mode first,
> and if you must test against live, use the smallest amount MarzPay permits.

| Provider | Sandbox | Without an explicit `base_url` you reach |
| --- | --- | --- |
| [`mtn`](mtn/) | Yes | Sandbox. Live needs explicit `base_url` **and** `target_environment` |
| [`airtel`](airtel/) | Yes (UAT) | Whichever `environment` selects |
| [`yopayments`](yopayments/) | Yes | Whichever `environment` selects |
| [`flutterwave`](flutterwave/) | Yes | Whichever `environment` selects |
| [`marzpay`](marzpay/) | **None** | **The live API, always** |

Use sandbox wherever one exists. When testing live, use the smallest amount the provider allows and your own number as both payer and recipient.

## Never put credentials in a report

Do not paste `api_key`, `api_secret`, `client_secret`, `password`, `pin`, `encrypted_pin`, or any webhook secret into an issue, discussion, or screenshot.

Error messages are safe — Momobase redacts secrets from upstream responses before they reach an error. If you are unsure whether something is sensitive, leave it out and say so.

## What we most need

Ranked. The first two are where real accounts tell us things we cannot learn any other way.

**1. Countries outside the supported list.** Number normalization currently recognises Uganda, Kenya, Rwanda, Tanzania, Zambia, Cameroon, DR Congo, Ghana, Malawi, Ethiopia, Côte d'Ivoire, Senegal and Nigeria. For any other country, `mtn`, `yopayments` and `marzpay` forward the number in whatever shape it arrived, without converting it to E.164. If you are in a market not listed, report the number you entered and the number the provider says it received.

**2. Any status reported as `unknown`.** Each adapter maps the provider's status vocabulary onto Momobase's, and anything unrecognised becomes `unknown`. Every one you find is a one-line fix, but only real traffic surfaces them. Include the raw provider status string.

**3. Airtel PIN disbursement and request signing.** Airtel disbursement encrypts a PIN with Airtel's RSA key, and `sign_requests` adds AES/RSA request signing. This code has no test coverage and has likely never run against Airtel in production. If you can exercise either path, we want to hear about it either way.

**4. MTN per-market settings.** Outside sandbox, MTN needs a market-specific `base_url` and `target_environment`. We cannot determine these without market access — if you have them working, tell us the pair.

**5. Verified webhooks.** `marzpay` and `flutterwave` verify signatures. Real signed payloads are the only way to confirm the verification matches what the provider actually sends.

## What to exercise

For whichever provider you hold an account with:

- **Collection** — a payment request to a number you control
- **Disbursement** — a payout to a number you control
- **Transaction query** — look the transaction up afterwards and check the status matches reality
- **Webhook** — if the provider supports it, confirm the callback arrives and verifies
- **Failure paths** — wrong number, insufficient balance, cancelled prompt, expired request

Reports on failures are as valuable as reports on success. A provider's error shape is exactly what we cannot guess.

## How to report

Use the [provider report template](../../issues/new?template=provider-report.yml). It asks for provider, country, currency, operation, environment, what you expected, what happened, and the resulting Momobase status.

One report per provider per issue, please — it keeps the follow-up fixes separable.

## Coverage

Fill in as reports arrive.

| Provider | Country | Collection | Disbursement | Query | Webhook | Reporter |
| --- | --- | --- | --- | --- | --- | --- |
| | | | | | | |
