package clarify

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/readhelper"
	"github.com/amp-labs/connectors/common/urlbuilder"
)

const (
	apiVersion = "v1"

	// List endpoints take page[limit] up to 500 (422 above it); the list references default to 500.
	// https://developer.clarify.ai/docs/api-basics/pagination
	maxPageSize = 500

	pageLimitParam  = "page[limit]"
	pageOffsetParam = "page[offset]"

	// Date operators on the record's `_updated_at`, case-sensitive, with the date values the docs
	// show (`filter[_created_at][Is after]=2024-01-01`).
	// https://developer.clarify.ai/docs/api-basics/filtering#operators-by-field-type
	updatedFromParam = "filter[_updated_at][Is on or after]"
	updatedToParam   = "filter[_updated_at][Is on or before]"

	// The server filter only narrows the pages; parseReadResponse applies the exact (Since, Until].
	// Live, `[Is after]=2025-08-27T18:36:13.720Z` returned a record updated at exactly
	// 2025-08-27T18:36:13.720Z, so the API does not compare at the timestamp's precision, and the
	// docs don't say what precision or timezone it uses. The window is widened by a day on each
	// side so that a comparison by date in any timezone can only return extra records, never drop one.
	filterMargin = 24 * time.Hour
)

// buildReadRequest lists records with page[limit]/page[offset] and follows links.next.
// Built-in objects are pre-filtered on `_updated_at` by the server, by date; every object,
// users included, is filtered exactly in parseReadResponse.
func (c *Connector) buildReadRequest(ctx context.Context, params common.ReadParams) (*http.Request, error) {
	readURL, err := c.readURL(params)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, http.MethodGet, readURL, nil)
}

func (c *Connector) readURL(params common.ReadParams) (string, error) {
	// NextPage is links.next on the connector's origin; its encoded query (filters included)
	// is used unchanged. See nextPage.
	if params.NextPage != "" {
		return params.NextPage.String(), nil
	}

	readURL, err := c.workspaceURL(readPath(params.ObjectName)...)
	if err != nil {
		return "", err
	}

	pageSize := min(readhelper.PageSizeWithDefault(params, maxPageSize), maxPageSize)
	readURL.WithQueryParam(pageLimitParam, strconv.Itoa(pageSize))
	readURL.WithQueryParam(pageOffsetParam, "0")

	if entityObjects.Has(params.ObjectName) {
		if !params.Since.IsZero() {
			readURL.WithQueryParam(updatedFromParam, params.Since.UTC().Add(-filterMargin).Format(time.DateOnly))
		}

		if !params.Until.IsZero() {
			readURL.WithQueryParam(updatedToParam, params.Until.UTC().Add(filterMargin).Format(time.DateOnly))
		}
	}

	return readURL.String(), nil
}

// readPath is the list endpoint below /v1/workspaces/{workspace}.
// Built-in objects: GET /objects/{object}/resources
// https://developer.clarify.ai/docs/api-reference/resources/getResources
// Users: GET /users
// https://developer.clarify.ai/docs/api-reference/users/getUsers
func readPath(objectName string) []string {
	if objectName == objectNameUser {
		return []string{"users"}
	}

	return []string{"objects", objectName, "resources"}
}

func (c *Connector) workspaceURL(path ...string) (*urlbuilder.URL, error) {
	segments := append([]string{apiVersion, "workspaces", c.workspace}, path...)

	result, err := urlbuilder.New(c.ProviderInfo().BaseURL, segments...)
	if err != nil {
		return nil, err
	}

	// Operator names contain spaces, which the docs encode as %20 (`Greater%20than`).
	// Literal `+` is encoded as %2B, so only spaces are affected.
	result.AddEncodingExceptions(map[string]string{"+": "%20"})

	return result, nil
}

func (c *Connector) parseReadResponse(
	ctx context.Context,
	params common.ReadParams,
	request *http.Request,
	response *common.JSONHTTPResponse,
) (*common.ReadResult, error) {
	return common.ParseResultFiltered(params, response,
		common.MakeRecordsFunc("data"),
		makeUpdatedAtFilter(c.nextPage),
		readhelper.MakeMarshaledDataFuncWithId(flattenRecord, readhelper.NewIdField("id")),
		params.Fields,
	)
}
