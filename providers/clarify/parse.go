package clarify

import (
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/spyzhov/ajson"
)

// getNextRecordsURL returns `links.next`, which is null on the last page.
// https://developer.clarify.ai/docs/api-basics/pagination#response
func getNextRecordsURL(node *ajson.Node) (string, error) {
	return jsonquery.New(node, "links").StrWithDefault("next", "")
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
