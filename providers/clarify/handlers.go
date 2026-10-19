package clarify

import (
	"context"
	"net/http"
	"strconv"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/readhelper"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/datautils"
)

const (
	apiVersion = "v1"

	// List endpoints take `page[limit]` up to 500 and `page[offset]`.
	// https://developer.clarify.ai/docs/api-basics/pagination
	maxPageSize     = 500
	pageLimitParam  = "page[limit]"
	pageOffsetParam = "page[offset]"

	// Datetime filters name the operator in a second bracket.
	// https://developer.clarify.ai/docs/api-basics/filtering#operators-by-field-type
	updatedSinceParam = "filter[_updated_at][Is on or after]"
	updatedUntilParam = "filter[_updated_at][Is on or before]"
)

// Operator names contain spaces, which Clarify expects as %20 rather than `+`.
// url.Values.Encode writes a literal `+` as %2B, so every `+` in the encoded query is a space.
// https://developer.clarify.ai/docs/api-basics/filtering#filter-with-an-operator
var spaceEncoding = map[string]string{"+": "%20"} // nolint:gochecknoglobals

func (c *Connector) buildReadRequest(ctx context.Context, params common.ReadParams) (*http.Request, error) {
	readURL, err := c.readURL(params)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, http.MethodGet, readURL, nil)
}

// readURL lists GET /v1/workspaces/{workspace}/objects/{object}/resources from offset 0.
// Later pages come from `links.next` (see makeNextPageFunc), which carries every query parameter.
//
// Since and Until are sent with inclusive operators. The docs show datetime filters only with
// whole dates and do not say whether a timestamp is compared at full precision, so the server
// filter narrows the result and parseReadResponse applies the exact (Since, Until] window.
func (c *Connector) readURL(params common.ReadParams) (string, error) {
	if params.NextPage != "" {
		return params.NextPage.String(), nil
	}

	readURL, err := urlbuilder.New(c.ProviderInfo().BaseURL,
		apiVersion, "workspaces", c.workspace, "objects", params.ObjectName, "resources")
	if err != nil {
		return "", err
	}

	readURL.AddEncodingExceptions(spaceEncoding)
	readURL.WithQueryParam(pageLimitParam,
		strconv.Itoa(min(readhelper.PageSizeWithDefault(params, maxPageSize), maxPageSize)))
	readURL.WithQueryParam(pageOffsetParam, "0")

	if !params.Since.IsZero() {
		readURL.WithQueryParam(updatedSinceParam, datautils.Time.FormatRFC3339inUTCWithMilliseconds(params.Since))
	}

	if !params.Until.IsZero() {
		readURL.WithQueryParam(updatedUntilParam, datautils.Time.FormatRFC3339inUTCWithMilliseconds(params.Until))
	}

	return readURL.String(), nil
}

func (c *Connector) parseReadResponse(
	ctx context.Context,
	params common.ReadParams,
	request *http.Request,
	response *common.JSONHTTPResponse,
) (*common.ReadResult, error) {
	return common.ParseResultFiltered(params, response,
		common.MakeRecordsFunc("data"),
		makeUpdatedAtFilter(makeNextPageFunc(request.URL)),
		readhelper.MakeMarshaledDataFuncWithId(flattenRecord, readhelper.NewIdField("id")),
		params.Fields,
	)
}
