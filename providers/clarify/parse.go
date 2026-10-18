package clarify

import (
	"fmt"
	"net/url"
	"time"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/readhelper"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/spyzhov/ajson"
)

// nextPage returns links.next, which is null on the last page, on the connector's origin.
// https://developer.clarify.ai/docs/api-basics/pagination#response
// The docs say to fetch the link verbatim, but the live API returns it with an `http://` scheme
// (http://api.clarify.ai/v1/workspaces/...), which would send the API key over plain HTTP.
// The link's path and already-encoded query are kept as they are; only scheme and host are replaced.
func (c *Connector) nextPage(node *ajson.Node) (string, error) {
	next, err := jsonquery.New(node, "links").StringOptional("next")
	if err != nil {
		return "", err
	}

	if next == nil || *next == "" {
		return "", nil
	}

	nextURL, err := url.Parse(*next)
	if err != nil {
		return "", err
	}

	baseURL, err := url.Parse(c.ProviderInfo().BaseURL)
	if err != nil {
		return "", err
	}

	nextURL.Scheme = baseURL.Scheme
	nextURL.Host = baseURL.Host

	return nextURL.String(), nil
}

// makeUpdatedAtFilter keeps records whose `_updated_at` falls within (Since, Until].
// It serves GET /users, whose reference documents only page and sortOrder, no filter.
// Users come back ordered by `_id`, so every page is read: paging cannot stop early.
// https://developer.clarify.ai/docs/api-reference/users/getUsers
func makeUpdatedAtFilter(next common.NextPageFunc) common.RecordsFilterFunc {
	boundary := readhelper.NewTimeBoundary(readhelper.ExcludeSince())

	return func(params common.ReadParams, body *ajson.Node, records []*ajson.Node) ([]*ajson.Node, string, error) {
		nextPageURL, err := next(body)
		if err != nil {
			return nil, "", err
		}

		if params.Since.IsZero() && params.Until.IsZero() {
			return records, nextPageURL, nil
		}

		filtered := make([]*ajson.Node, 0, len(records))

		for _, record := range records {
			timestamp, err := updatedAt(record)
			if err != nil {
				return nil, "", err
			}

			if boundary.Contains(params, timestamp) {
				filtered = append(filtered, record)
			}
		}

		return filtered, nextPageURL, nil
	}
}

func updatedAt(record *ajson.Node) (time.Time, error) {
	value, err := jsonquery.New(record, "attributes").StringOptional("_updated_at")
	if err != nil {
		return time.Time{}, err
	}

	if value == nil {
		return time.Time{}, fmt.Errorf("%w: _updated_at", common.ErrMissingExpectedValues)
	}

	return time.Parse(time.RFC3339, *value)
}

// flattenRecord moves a JSON:API resource's attributes to the top level:
//
//	{"type": "person", "id": "5f8b...", "attributes": {"_id": "5f8b...", "job_title": "VP"}}
//
// becomes {"type": "person", "id": "5f8b...", "_id": "5f8b...", "job_title": "VP"}.
// Attribute keys are kept verbatim, as GET /schemas names them. On a clash the attribute wins,
// because metadata describes attributes (a schema may define its own `type` field), except for
// the resource `id`, which identifies the record. The shadowed value stays available in Raw.
func flattenRecord(node *ajson.Node) (map[string]any, error) {
	record, err := jsonquery.Convertor.ObjectToMap(node)
	if err != nil {
		return nil, err
	}

	attributes, _ := record["attributes"].(map[string]any)
	delete(record, "attributes")

	for key, value := range attributes {
		if key == "id" {
			continue
		}

		record[key] = value
	}

	return record, nil
}
