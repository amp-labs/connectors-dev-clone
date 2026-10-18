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

	// List endpoints default to 20 records and accept at most 100.
	// https://docs.withorb.com/api-reference/pagination#limits
	maxPageSize = 100
)

// buildReadRequest lists an object with cursor pagination.
//
// Since and Until are not sent. None of these list endpoints filters on an update time, and none of
// the records carries one: customers, plans and credit notes filter only on created_at, invoices on
// invoice_date and due_date, and items and coupons not at all. A created_at window would never
// return a record changed after it was created (an invoice that was paid, a customer whose email
// changed, a plan or coupon that was archived), so every read returns the full list.
// https://docs.withorb.com/api-reference/customer/list-customers
// https://docs.withorb.com/api-reference/invoice/list-invoices
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

	// The coupon list returns only active coupons unless archived ones are asked for.
	// https://docs.withorb.com/api-reference/coupon/list-coupons
	if params.ObjectName == objectNameCoupons {
		readURL.WithQueryParam("show_archived", "true")
	}

	return readURL, nil
}

func (c *Connector) parseReadResponse(
	ctx context.Context,
	params common.ReadParams,
	request *http.Request,
	response *common.JSONHTTPResponse,
) (*common.ReadResult, error) {
	return common.ParseResult(
		response,
		common.MakeRecordsFunc("data"),
		makeNextPageFunc(request.URL),
		readhelper.MakeMarshaledDataFuncWithId(nil, readhelper.NewIdField("id")),
		params.Fields,
	)
}
