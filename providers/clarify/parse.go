package clarify

import (
	"time"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/readhelper"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/spyzhov/ajson"
)

// getNextRecordsURL returns `links.next`, which is null on the last page.
// https://developer.clarify.ai/docs/api-basics/pagination#response
func getNextRecordsURL(node *ajson.Node) (string, error) {
	return jsonquery.New(node, "links").StrWithDefault("next", "")
}

// makeUpdatedAtFilter keeps users whose `_updated_at` falls within (Since, Until].
// GET /users documents only page and sortOrder parameters, no filter, so the window is applied here
// and every page is read.
// https://developer.clarify.ai/docs/api-reference/users/getUsers
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
// Attribute values are kept as returned, so `name` stays {first_name, last_name} and collection
// fields stay {"items": [...]}, the shapes the schemas endpoint describes.
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
