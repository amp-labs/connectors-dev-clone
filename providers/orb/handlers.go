package orb

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/readhelper"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/datautils"
	"github.com/amp-labs/connectors/providers/orb/internal/metadata"
)

const (
	apiVersion = "v1"

	// Page size defaults to 20 and is at most 100.
	// https://docs.withorb.com/api-reference/pagination#limits
	maxPageSize = 100
)

// buildReadRequest lists records with cursor pagination. Every page repeats the same limit and
// time filters, with the previous page's next_cursor as `cursor`.
// https://docs.withorb.com/api-reference/pagination
func (c *Connector) buildReadRequest(ctx context.Context, params common.ReadParams) (*http.Request, error) {
	path, err := metadata.Catalog.ReadPath(params.ObjectName)
	if err != nil {
		return nil, err
	}

	readURL, err := urlbuilder.New(c.ProviderInfo().BaseURL, apiVersion, path)
	if err != nil {
		return nil, err
	}

	readURL.WithQueryParam("limit", strconv.Itoa(min(readhelper.PageSizeWithDefault(params, maxPageSize), maxPageSize)))

	if params.NextPage != "" {
		readURL.WithQueryParam("cursor", params.NextPage.String())
	}

	// Since is exclusive and Until inclusive, as ReadParams define them.
	if field, ok := timeFilterField[params.ObjectName]; ok {
		if !params.Since.IsZero() {
			readURL.WithQueryParam(field+"[gt]", formatTime(params.Since))
		}

		if !params.Until.IsZero() {
			readURL.WithQueryParam(field+"[lte]", formatTime(params.Until))
		}
	}

	return http.NewRequestWithContext(ctx, http.MethodGet, readURL.String(), nil)
}

// formatTime keeps milliseconds so that a whole-second bound does not drop records
// created within the last second before Until.
func formatTime(t time.Time) string {
	return datautils.Time.FormatRFC3339inUTCWithMilliseconds(t)
}

func (c *Connector) parseReadResponse(
	ctx context.Context,
	params common.ReadParams,
	request *http.Request,
	response *common.JSONHTTPResponse,
) (*common.ReadResult, error) {
	filter := readhelper.MakeIdentityFilterFunc(getNextCursor)

	if _, ok := timeFilterField[params.ObjectName]; !ok {
		// Items are listed newest first by created_at, so paging stops at the first record
		// created at or before Since.
		// https://docs.withorb.com/api-reference/item/list-items
		filter = readhelper.MakeTimeFilterFunc(
			readhelper.ReverseOrder,
			readhelper.NewTimeBoundary(readhelper.ExcludeSince()),
			"created_at", time.RFC3339,
			getNextCursor,
		)
	}

	return common.ParseResultFiltered(params, response,
		common.MakeRecordsFunc("data"),
		filter,
		readhelper.MakeMarshaledDataFuncWithId(nil, readhelper.NewIdField("id")),
		params.Fields,
	)
}
