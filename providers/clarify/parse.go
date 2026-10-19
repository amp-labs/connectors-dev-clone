package clarify

import (
	"net/url"
	"time"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/readhelper"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/spyzhov/ajson"
)

// makeNextPageFunc follows `links.next`, which is null on the last page and percent-encodes every
// query parameter of the request, filters included.
// https://developer.clarify.ai/docs/api-basics/pagination#response
//
// The live API returns the link with an `http://` scheme, and plain HTTP to api.clarify.ai times out,
// so the link's path and query are kept verbatim on the scheme and host of the request that was sent.
func makeNextPageFunc(requestURL *url.URL) common.NextPageFunc {
	return func(node *ajson.Node) (string, error) {
		next, err := jsonquery.New(node, "links").StrWithDefault("next", "")
		if err != nil || next == "" {
			return "", err
		}

		nextURL, err := url.Parse(next)
		if err != nil {
			return "", err
		}

		nextURL.Scheme = requestURL.Scheme
		nextURL.Host = requestURL.Host

		return nextURL.String(), nil
	}
}

// makeUpdatedAtFilter keeps records whose `_updated_at` falls within (Since, Until], matching ReadParams:
// Since is exclusive and Until inclusive. Records are ordered by `_id`, not by time, so every page is read.
// https://developer.clarify.ai/docs/api-basics/pagination#ordering
func makeUpdatedAtFilter(nextPage common.NextPageFunc) common.RecordsFilterFunc {
	return readhelper.MakeTimeFilterFuncWithZoom(
		readhelper.Unordered,
		readhelper.NewTimeBoundary(readhelper.ExcludeSince()),
		[]string{"attributes"}, "_updated_at", time.RFC3339,
		nextPage,
	)
}

// flattenRecord promotes a JSON:API resource's attributes to top-level fields and keeps its id:
//
//	{"type": "person", "id": "5f8b...", "attributes": {"job_title": "VP", "_updated_at": "..."}}
//
// becomes {"id": "5f8b...", "job_title": "VP", "_updated_at": "..."}.
// Attribute values are kept as returned, so `name` stays {first_name, last_name, full_name} and
// collection fields stay {"items": [...]}, the shapes the schemas endpoint describes.
func flattenRecord(node *ajson.Node) (map[string]any, error) {
	attributes, err := jsonquery.New(node).ObjectOptional("attributes")
	if err != nil {
		return nil, err
	}

	record := make(map[string]any)

	if attributes != nil {
		record, err = jsonquery.Convertor.ObjectToMap(attributes)
		if err != nil {
			return nil, err
		}
	}

	id, err := jsonquery.New(node).StringOptional("id")
	if err != nil {
		return nil, err
	}

	if id != nil {
		record["id"] = *id
	}

	return record, nil
}
