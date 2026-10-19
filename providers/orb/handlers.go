package orb

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

	// List endpoints default to 20 records and accept at most 100.
	// https://docs.withorb.com/api-reference/pagination#limits
	maxPageSize = 100

	// Since is exclusive and Until inclusive, matching ReadParams.
	// created_at is stored below the second (the API prints whole seconds), so the bounds keep milliseconds.
	// https://docs.withorb.com/api-reference/customer/list-customers
	createdSinceParam = "created_at[gt]"
	createdUntilParam = "created_at[lte]"
)

// buildReadRequest lists records from GET /v1/{object}. NextPage is the full URL of the
// next page, which carries the cursor and the filters of the first request.
// https://docs.withorb.com/api-reference/pagination
func (c *Connector) buildReadRequest(ctx context.Context, params common.ReadParams) (*http.Request, error) {
	readURL, err := c.readURL(params)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, http.MethodGet, readURL.String(), nil)
}

func (c *Connector) readURL(params common.ReadParams) (*urlbuilder.URL, error) {
	if params.NextPage != "" {
		return urlbuilder.New(params.NextPage.String())
	}

	readURL, err := urlbuilder.New(c.ProviderInfo().BaseURL, apiVersion, params.ObjectName)
	if err != nil {
		return nil, err
	}

	readURL.WithQueryParam("limit", strconv.Itoa(min(readhelper.PageSizeWithDefault(params, maxPageSize), maxPageSize)))

	if objectsWithCreatedAtFilter.Has(params.ObjectName) {
		if !params.Since.IsZero() {
			readURL.WithQueryParam(createdSinceParam, datautils.Time.FormatRFC3339inUTCWithMilliseconds(params.Since))
		}

		if !params.Until.IsZero() {
			readURL.WithQueryParam(createdUntilParam, datautils.Time.FormatRFC3339inUTCWithMilliseconds(params.Until))
		}
	}

	return readURL, nil
}

func (c *Connector) parseReadResponse(
	ctx context.Context,
	params common.ReadParams,
	request *http.Request,
	response *common.JSONHTTPResponse,
) (*common.ReadResult, error) {
	return common.ParseResult(response,
		common.MakeRecordsFunc("data"),
		makeNextPageFunc(request.URL),
		readhelper.MakeMarshaledDataFuncWithId(nil, readhelper.NewIdField("id")),
		params.Fields,
	)
}
