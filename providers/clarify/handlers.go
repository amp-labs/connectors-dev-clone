package clarify

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/readhelper"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/datautils"
)

const (
	apiVersion = "v1"

	// Most list endpoints accept `page[limit]` up to 500 and return 422 above it.
	// https://developer.clarify.ai/docs/api-basics/pagination
	maxPageSize = 500

	// `_updated_at` is a datetime, so it takes the date operators; Since is exclusive and Until
	// inclusive, as in ReadParams.
	// https://developer.clarify.ai/docs/api-basics/filtering#operators-by-field-type
	updatedSinceParam = "filter[_updated_at][Is after]"
	updatedUntilParam = "filter[_updated_at][Is on or before]"

	updatedAtField = "_updated_at"
)

// Operator names contain spaces, which the docs ask to be sent as %20.
var queryEncodingExceptions = map[string]string{"+": "%20"} // nolint:gochecknoglobals

// buildReadRequest lists records. Built-in objects use the resources endpoint, which filters
// Since/Until on `_updated_at`; users use the users endpoint, which takes no filter and is filtered
// in parseReadResponse. Later pages are the provider's `links.next` URL (see makeNextRecordsURL):
// it already carries the page and filter parameters.
// https://developer.clarify.ai/docs/api-reference/resources/getResources
// https://developer.clarify.ai/docs/api-reference/users/getUsers
func (c *Connector) buildReadRequest(ctx context.Context, params common.ReadParams) (*http.Request, error) {
	if params.NextPage != "" {
		return http.NewRequestWithContext(ctx, http.MethodGet, params.NextPage.String(), nil)
	}

	readURL, err := c.objectURL(params.ObjectName)
	if err != nil {
		return nil, err
	}

	readURL.WithQueryParam("page[limit]",
		strconv.Itoa(min(readhelper.PageSizeWithDefault(params, maxPageSize), maxPageSize)))

	if entityObjects.Has(params.ObjectName) {
		if !params.Since.IsZero() {
			readURL.WithQueryParam(updatedSinceParam, datautils.Time.FormatRFC3339inUTCWithMilliseconds(params.Since))
		}

		if !params.Until.IsZero() {
			readURL.WithQueryParam(updatedUntilParam, datautils.Time.FormatRFC3339inUTCWithMilliseconds(params.Until))
		}
	}

	readURL.AddEncodingExceptions(queryEncodingExceptions)

	return http.NewRequestWithContext(ctx, http.MethodGet, readURL.String(), nil)
}

func (c *Connector) objectURL(objectName string) (*urlbuilder.URL, error) {
	if objectName == objectNameUser {
		return c.workspaceURL("users")
	}

	return c.workspaceURL("objects", objectName, "resources")
}

func (c *Connector) workspaceURL(path ...string) (*urlbuilder.URL, error) {
	return urlbuilder.New(c.ProviderInfo().BaseURL,
		append([]string{apiVersion, "workspaces", c.Workspace}, path...)...)
}

func (c *Connector) parseReadResponse(
	ctx context.Context,
	params common.ReadParams,
	request *http.Request,
	response *common.JSONHTTPResponse,
) (*common.ReadResult, error) {
	nextPage := makeNextRecordsURL(request.URL)
	filter := readhelper.MakeIdentityFilterFunc(nextPage)

	if !entityObjects.Has(params.ObjectName) {
		// Users come back ordered by `_id`, so every page is read and filtered.
		filter = readhelper.MakeTimeFilterFuncWithZoom(
			readhelper.Unordered,
			readhelper.NewTimeBoundary(readhelper.ExcludeSince()),
			[]string{"attributes"}, updatedAtField, time.RFC3339,
			nextPage,
		)
	}

	return common.ParseResultFiltered(params, response,
		common.MakeRecordsFunc("data"),
		filter,
		readhelper.MakeMarshaledDataFuncWithId(flattenRecord, readhelper.NewIdField("id")),
		params.Fields,
	)
}
