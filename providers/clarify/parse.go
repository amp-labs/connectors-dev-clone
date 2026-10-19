package clarify

import (
	"fmt"
	"net/url"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/spyzhov/ajson"
)

// makeNextRecordsURL returns `links.next`, which is null on the last page, moved onto the scheme
// and host of the request that returned it. The live API returns these links as `http://`,
// although the docs show `https://`, and port 80 does not answer. Path and query are kept as they
// are: they carry the page and filter parameters.
// https://developer.clarify.ai/docs/api-basics/pagination#response
func makeNextRecordsURL(requestURL *url.URL) common.NextPageFunc {
	return func(node *ajson.Node) (string, error) {
		next, err := jsonquery.New(node, "links").StrWithDefault("next", "")
		if err != nil || next == "" {
			return "", err
		}

		nextURL, err := url.Parse(next)
		if err != nil {
			return "", fmt.Errorf("invalid links.next %q: %w", next, err)
		}

		nextURL.Scheme = requestURL.Scheme
		nextURL.Host = requestURL.Host

		return nextURL.String(), nil
	}
}

// flattenRecord turns a JSON:API resource into flat fields:
//
//	{"type": "person", "id": "5f8b...", "attributes": {"job_title": "VP", "_updated_at": "..."}}
//
// becomes {"id": "5f8b...", "job_title": "VP", "_updated_at": "..."}.
// Attribute values are kept as they are, so collections stay `{"items": [...]}` and names stay
// `{"first_name": ..., "last_name": ...}`, the shapes Clarify writes accept.
// The resource `id` wins over an attribute of the same name; Raw keeps the untouched resource.
func flattenRecord(node *ajson.Node) (map[string]any, error) {
	record, err := jsonquery.New(node).ObjectOptional("attributes")
	if err != nil {
		return nil, err
	}

	fields := make(map[string]any)

	if record != nil {
		fields, err = jsonquery.Convertor.ObjectToMap(record)
		if err != nil {
			return nil, err
		}
	}

	id, err := jsonquery.New(node).TextWithDefault("id", "")
	if err != nil {
		return nil, err
	}

	fields["id"] = id

	return fields, nil
}
