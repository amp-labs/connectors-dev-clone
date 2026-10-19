# Clarify connector

Read-only deep connector for the Clarify CRM REST API (JSON:API).

## Auth and requests

- API key from Settings > API Keys, sent as `Authorization: api-key <key>`, not `Bearer`
  ([Authentication](https://developer.clarify.ai/docs/getting-started/authentication#api-keys)).
- Base URL `https://api.clarify.ai`. Every path is `/v1/workspaces/{workspace}/...`, so the catalog
  declares a `workspace` metadata input (the workspace slug). It reaches the connector as
  `ConnectorParams.Workspace` and is required. Proxy callers put `/v1/workspaces/<slug>` in their
  own paths.

## Objects

| Object    | Endpoint                                              | Metadata source                     |
|-----------|-------------------------------------------------------|-------------------------------------|
| `person`  | `GET /v1/workspaces/{workspace}/objects/person/resources`  | `GET /v1/workspaces/{workspace}/schemas` |
| `company` | `GET /v1/workspaces/{workspace}/objects/company/resources` | same                                |
| `deal`    | `GET /v1/workspaces/{workspace}/objects/deal/resources`    | same                                |
| `meeting` | `GET /v1/workspaces/{workspace}/objects/meeting/resources` | same                                |
| `task`    | `GET /v1/workspaces/{workspace}/objects/task/resources`    | same                                |
| `user`    | `GET /v1/workspaces/{workspace}/users`                | static, from the users endpoint docs |

Object names are Clarify's own entity names (the `{object}` path parameter and the JSON:API
`type`); users have no `{object}` path, and `user` is their JSON:API type.

The schemas endpoint returns every object's JSON Schema in one call, including custom fields and
enum options. `.../entities/{object}` schemas are objects; `.../core/*` ones are skipped. Enums map
to `singleSelect`, many-to-one/one-to-one relationships to `reference`, to-many relationships
(`{"items": [...]}` collections) to `other`. The record system fields `_id`, `_created_at`,
`_updated_at`, `_created_by` and `_updated_by` are always listed, plus the resource `id`.

## Record shape

Read flattens `data[].attributes` to top-level fields and adds the resource `id`. Values are kept
as Clarify returns them: collections stay `{"items": [...]}` and person names stay
`{"first_name", "last_name"}`, the same shapes Clarify's writes take. `Raw` is untouched.

## Pagination

`page[limit]` (max 500, the default here) and `links.next`, which is `null` on the last page.
`links.next` is fetched verbatim; it carries the page and filter parameters
([Pagination](https://developer.clarify.ai/docs/api-basics/pagination)).

## Incremental read

The five entity objects filter on the server: `filter[_updated_at][Is after]` for Since and
`filter[_updated_at][Is on or before]` for Until, RFC 3339 UTC with milliseconds, spaces sent as
`%20` ([Filtering](https://developer.clarify.ai/docs/api-basics/filtering)).
The users endpoint documents no filter, so users are read in full and kept when `_updated_at` is
in `(Since, Until]`.

## Rate limits

3,000 requests per minute, per workspace, per endpoint, rolling 60-second window; 429 carries
`Retry-After` ([Rate limits](https://developer.clarify.ai/docs/api-basics/rate-limits)).
