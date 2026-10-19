package orb

import (
	"context"
	"net/http"
	"strconv"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/readhelper"
	"github.com/amp-labs/connectors/common/urlbuilder"
)

const (
	apiVersion = "v1"

	// Page size defaults to 20 and is capped at 100.
	// https://docs.withorb.com/api-reference/pagination#limits
	maxPageSize = 100
)

// buildReadRequest lists an object's records, newest created first, with cursor pagination.
//
// Since and Until are not sent: no list endpoint filters on update time and the records carry
// no update timestamp. customers, plans and credit_notes accept created_at[...] and invoices
// invoice_date[...], but filtering on those would drop every record changed after it was
// created (an invoice moving from draft to paid, an edited customer), so every read is a full read.
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

	pageSize := min(readhelper.PageSizeWithDefault(params, maxPageSize), maxPageSize)
	readURL.WithQueryParam("limit", strconv.Itoa(pageSize))

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
