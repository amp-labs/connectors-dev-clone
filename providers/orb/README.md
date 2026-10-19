# Orb connector

Read-only deep connector for [Orb](https://docs.withorb.com), the usage-based billing platform.

## Auth and base URL

An Orb API key (Orb > Developers > API keys), sent as `Authorization: Bearer <key>`.
The catalog base URL is `https://api.withorb.com`; the connector adds the `/v1` prefix.

## Objects

Every object is read from `GET /v1/{object}`:

| Object          | Incremental read                         |
|-----------------|------------------------------------------|
| `customers`     | `created_at[gt]` / `created_at[lte]`     |
| `subscriptions` | `created_at[gt]` / `created_at[lte]`     |
| `plans`         | `created_at[gt]` / `created_at[lte]`     |
| `credit_notes`  | `created_at[gt]` / `created_at[lte]`     |
| `invoices`      | none, always a full read                 |
| `items`         | none, always a full read                 |
| `coupons`       | none, always a full read                 |

Orb records have no `updated_at`, and no list endpoint filters on modification time, so
creation time is the only server-side window. An incremental read of a created_at-filtered
object returns records **created** in the window: later changes to an older customer or
subscription (status, plan, email...) are not picked up until a full read.
Invoices filter only on `invoice_date` / `due_date` (not creation or modification), and items
and coupons have no time filter, so `Since`/`Until` are ignored for them.

The subscriptions `created_at` filter is not on any page of the API reference we had, but
was confirmed live: a `created_at[gte]` in 2100 returns no records, and a `(gt, lte]` window
around a known record returns it. `created_at` is stored below the second (the API prints whole
seconds), so the bounds are sent with milliseconds.

## Pagination

Cursor-based: `limit` (max 100, the default here) and `cursor`, from
`pagination_metadata.next_cursor` while `pagination_metadata.has_more` is true.
`NextPage` is the full next URL, so filters carry over.

## Metadata

`internal/metadata/objectsMetadata.json` is written by hand from the example list responses in
Orb's API reference (Orb publishes no OpenAPI spec). Types come from the examples; `status`
on invoices and plans is a single select with the values the list filters document.
Subscriptions have no schema page, so their fields are sampled from one live record
(`GET /v1/subscriptions?limit=1`); an account with no subscriptions reports an error for that
object only.

Fields are the top-level keys of each record; nested objects (`customer`, `plan`, addresses,
line items) are returned as they are.

## Errors and rate limits

Errors are RFC 7807 (`type`, `status`, `title`, `detail`), mapped by `errors.go`.
GET requests are limited to 100 rps in live mode, 50 in test mode and 10 on trials
(https://docs.withorb.com/api-reference/rate-limits).
