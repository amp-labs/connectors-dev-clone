# Orb

Read connector for Orb's billing API.

## Auth and base URL

API key sent as `Authorization: Bearer <key>`. Keys are issued in the Orb dashboard
(Developers > API keys). Base URL `https://api.withorb.com`; every path is under `/v1`.

## Objects

`customers`, `subscriptions`, `invoices`, `plans`, `items`, `credit_notes`, `coupons`, each read
from `GET /v1/{object}`. Coupons are read with `show_archived=true`, since the endpoint otherwise
hides archived coupons.

## Pagination

Cursor based: the response's `pagination_metadata.next_cursor` is sent back as `cursor` while
`pagination_metadata.has_more` is true. `limit` defaults to 100, the documented maximum.
https://docs.withorb.com/api-reference/pagination

## Incremental reads

Not supported: every read returns the full list, and `Since`/`Until` are ignored. No list endpoint
filters on an update time and no record carries one. Customers, plans and credit notes filter on
`created_at`, invoices on `invoice_date`/`due_date`, and items, coupons and subscriptions have no
documented time filter. A `created_at` window would never pick up a record changed after creation
(an invoice moving from issued to paid, an archived plan or coupon).

## Metadata

There is no describe endpoint and no OpenAPI spec was available, so
`internal/metadata/objectsMetadata.json` was written by hand from the 200 response examples of each
list endpoint's reference page. Enum values are known only for `invoices.status` and
`plans.status` (from the list endpoints' `status` filters). Nested objects and arrays are returned
as they are, typed `other`.

The docs carry no subscription schema, so `subscriptions` metadata is sampled from the newest
subscription (`GET /v1/subscriptions?limit=1`). An account with no subscriptions gets a metadata
error for that object.

## Rate limits

`GET` requests: 100/s in live mode, 50/s in test mode, 10/s on trials.
https://docs.withorb.com/api-reference/rate-limits
