package orb

import (
	"net/url"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/spyzhov/ajson"
)

// makeNextPageFunc repeats the request with `cursor` set to `pagination_metadata.next_cursor`
// while `has_more` is true.
// https://docs.withorb.com/api-reference/pagination#paginated-api-responses
func makeNextPageFunc(requestURL *url.URL) common.NextPageFunc {
	return func(node *ajson.Node) (string, error) {
		pagination, err := jsonquery.New(node).ObjectRequired("pagination_metadata")
		if err != nil {
			return "", err
		}

		query := jsonquery.New(pagination)

		hasMore, err := query.BoolWithDefault("has_more", false)
		if err != nil || !hasMore {
			return "", err
		}

		cursor, err := query.StringOptional("next_cursor")
		if err != nil || cursor == nil || *cursor == "" {
			return "", err
		}

		nextURL, err := urlbuilder.FromRawURL(requestURL)
		if err != nil {
			return "", err
		}

		nextURL.WithQueryParam("cursor", *cursor)

		return nextURL.String(), nil
	}
}
