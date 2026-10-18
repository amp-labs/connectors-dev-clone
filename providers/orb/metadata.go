package orb

import (
	"context"
	"slices"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/naming"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/amp-labs/connectors/providers/orb/internal/metadata"
)

// ListObjectMetadata serves the static catalog written from the docs' list-endpoint examples.
// Subscriptions are sampled from GET /v1/subscriptions instead.
// TODO: the docs carry no subscription schema (the List subscriptions page has no response example),
// so its fields are those of the newest subscription, and an account without subscriptions gets an error.
// https://docs.withorb.com/api-reference/subscription/list-subscriptions
func (c *Connector) ListObjectMetadata(
	ctx context.Context, objectNames []string,
) (*common.ListObjectMetadataResult, error) {
	result, err := metadata.Catalog.SelectResult(objectNames)
	if err != nil {
		return nil, err
	}

	if !slices.Contains(objectNames, objectNameSubscriptions) {
		return result, nil
	}

	delete(result.Errors, objectNameSubscriptions)

	subscriptions, err := c.sampleObjectMetadata(ctx, objectNameSubscriptions)
	if err != nil {
		result.AppendError(objectNameSubscriptions, err)
	} else {
		result.Result[objectNameSubscriptions] = *subscriptions
	}

	return result, nil
}

func (c *Connector) sampleObjectMetadata(ctx context.Context, objectName string) (*common.ObjectMetadata, error) {
	sampleURL, err := urlbuilder.New(c.ProviderInfo().BaseURL, apiVersion, objectName)
	if err != nil {
		return nil, err
	}

	sampleURL.WithQueryParam("limit", "1")

	response, err := c.JSONHTTPClient().Get(ctx, sampleURL.String())
	if err != nil {
		return nil, err
	}

	body, ok := response.Body()
	if !ok {
		return nil, common.ErrEmptyJSONHTTPResponse
	}

	records, err := jsonquery.New(body).ArrayRequired("data")
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, common.ErrMissingExpectedValues
	}

	record, err := jsonquery.Convertor.ObjectToMap(records[0])
	if err != nil {
		return nil, err
	}

	fields := make(common.FieldsMetadata, len(record))
	for name, value := range record {
		fields[name] = common.FieldMetadata{
			DisplayName: fieldDisplayName(name),
			ValueType:   common.InferValueTypeFromData(value),
		}
	}

	return common.NewObjectMetadata(naming.CapitalizeFirstLetterEveryWord(objectName), fields), nil
}

func fieldDisplayName(name string) string {
	return naming.CapitalizeFirstLetterEveryWord(strings.ReplaceAll(name, "_", " "))
}
