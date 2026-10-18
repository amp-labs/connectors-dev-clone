# Clarify connector

Read-only deep connector for the Clarify CRM REST API (JSON:API).

## Auth and requests

- API key from Settings > API Keys, sent as `Authorization: api-key <key>`, not Bearer
  ([Authentication](https://developer.clarify.ai/docs/getting-started/authentication)).
  A workspace key only sees public data; a personal key sees what its creator can.
- Base URL `https://api.clarify.ai`. Every path is under `/v1/workspaces/{workspace}/`, where
  `workspace` is the slug from the Clarify login URL (the `workspace` metadata input, which reaches
  the connector as `ConnectorParams.Workspace`). Proxy callers put the slug in their own paths.

## Objects

| Object    | Endpoint                                 | Metadata           | Incremental        |
|-----------|------------------------------------------|--------------------|--------------------|
| `person`  | `GET /objects/person/resources`          | `GET /schemas`     | server by date + connector |
| `company` | `GET /objects/company/resources`         | `GET /schemas`     | server by date + connector |
| `deal`    | `GET /objects/deal/resources`            | `GET /schemas`     | server by date + connector |
| `meeting` | `GET /objects/meeting/resources`         | `GET /schemas`     | server by date + connector |
| `task`    | `GET /objects/task/resources`            | `GET /schemas`     | server by date + connector |
| `user`    | `GET /users`                             | static (docs)      | connector only     |

Object names are the `{object}` path parameter. `GET /schemas` returns every schema of the
workspace in one listing, fetched once per `ListObjectMetadata` call; `.../entities/{object}`
schemas are used, and any `$ref`/`allOf` to `.../core/*` definitions is merged in. Custom fields
appear in those schemas and are listed like built-in ones. To-many relationship properties
(a person's `deals`) are not record attributes, so they are left out; to-one ones (`company_id`)
are `reference` fields. Users have no schema; their fields are those GET /users returns.

## Record shape

`attributes` are moved to the top level next to the resource `id` and `type`. On a clash the
attribute wins, except `id`. Collections stay as returned (`{"items": [...]}`), as do `name` and
location objects. `Raw` is the untouched resource.

## Pagination

`page[limit]` (max 500, the default here) and `page[offset]`; the next page is `links.next`, null
on the last page ([Pagination](https://developer.clarify.ai/docs/api-basics/pagination)).
The live API returns `links.next` as `http://api.clarify.ai/...`, so the connector keeps its path
and encoded query and puts it back on the `https` base URL. Without `sortOrder`, results are
ordered by `_id`.

## Incremental read

The connector keeps records whose `_updated_at` is within `(Since, Until]` for every object.
Built-in objects are also narrowed on the server with `filter[_updated_at][Is on or after]=<Since - 1 day>`
and `[Is on or before]=<Until + 1 day>`, as dates
([Filtering](https://developer.clarify.ai/docs/api-basics/filtering)). The server filter can't be
exact: live, `[Is after]=2025-08-27T18:36:13.720Z` returned a record updated at exactly that time,
and the docs give neither the precision nor the timezone of the comparison, so the day of margin
keeps it from dropping records. `GET /users` documents no filter, so users are read in full.

## Rate limits

3,000 requests per minute per workspace per endpoint, rolling 60 seconds; 429 carries
`Retry-After` ([Rate limits](https://developer.clarify.ai/docs/api-basics/rate-limits)).
