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

	// List endpoints take page[limit] up to 500 (422 above it); the list references default to 500.
	// https://developer.clarify.ai/docs/api-basics/pagination
	maxPageSize = 500

	pageLimitParam  = "page[limit]"
	pageOffsetParam = "page[offset]"

	// Datetime operators on the record's `_updated_at`, matching ReadParams:
	// Since is exclusive and Until inclusive. Operator names are case-sensitive.
	// https://developer.clarify.ai/docs/api-basics/filtering#operators-by-field-type
	updatedSinceParam = "filter[_updated_at][Is after]"
	updatedUntilParam = "filter[_updated_at][Is on or before]"
)

// buildReadRequest lists records with page[limit]/page[offset] and follows links.next verbatim.
// Built-in objects are filtered on `_updated_at` by the server; users are filtered in parseReadResponse.
func (c *Connector) buildReadRequest(ctx context.Context, params common.ReadParams) (*http.Request, error) {
	readURL, err := c.readURL(params)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, http.MethodGet, readURL, nil)
}

func (c *Connector) readURL(params common.ReadParams) (string, error) {
	// "Link URLs are percent-encoded ... Pass them through unchanged rather than rebuilding them."
	// https://developer.clarify.ai/docs/api-basics/pagination#response
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
			readURL.WithQueryParam(updatedSinceParam, datautils.Time.FormatRFC3339inUTCWithMilliseconds(params.Since))
		}

		if !params.Until.IsZero() {
			readURL.WithQueryParam(updatedUntilParam, datautils.Time.FormatRFC3339inUTCWithMilliseconds(params.Until))
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
	filter := readhelper.MakeIdentityFilterFunc(nextPage)
	if !entityObjects.Has(params.ObjectName) {
		filter = makeUpdatedAtFilter(nextPage)
	}

	return common.ParseResultFiltered(params, response,
		common.MakeRecordsFunc("data"),
		filter,
		readhelper.MakeMarshaledDataFuncWithId(flattenRecord, readhelper.NewIdField("id")),
		params.Fields,
	)
}
