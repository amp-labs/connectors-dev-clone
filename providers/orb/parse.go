package orb

import (
	"net/url"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/spyzhov/ajson"
)

// makeNextPageFunc repeats the request with `cursor` set to `pagination_metadata.next_cursor`
// while `pagination_metadata.has_more` is true.
// https://docs.withorb.com/api-reference/pagination#paginated-api-responses
func makeNextPageFunc(requestURL *url.URL) common.NextPageFunc {
	return func(node *ajson.Node) (string, error) {
		pagination := jsonquery.New(node, "pagination_metadata")

		hasMore, err := pagination.BoolWithDefault("has_more", false)
		if err != nil {
			return "", err
		}

		cursor, err := pagination.StrWithDefault("next_cursor", "")
		if err != nil {
			return "", err
		}

		if !hasMore || cursor == "" {
			return "", nil
		}

		nextURL, err := urlbuilder.FromRawURL(requestURL)
		if err != nil {
			return "", err
		}

		nextURL.WithQueryParam("cursor", cursor)

		return nextURL.String(), nil
	}
}
