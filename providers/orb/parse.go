package orb

import (
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/spyzhov/ajson"
)

// getNextCursor returns pagination_metadata.next_cursor while has_more is true.
// https://docs.withorb.com/api-reference/pagination#paginated-api-responses
func getNextCursor(node *ajson.Node) (string, error) {
	query := jsonquery.New(node, "pagination_metadata")

	hasMore, err := query.BoolWithDefault("has_more", false)
	if err != nil {
		return "", err
	}

	if !hasMore {
		return "", nil
	}

	return query.StrWithDefault("next_cursor", "")
}
