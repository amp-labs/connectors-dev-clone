package orb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/naming"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/amp-labs/connectors/providers/orb/internal/metadata"
)

// ListObjectMetadata serves the static catalog written from Orb's API reference.
// Subscriptions have no schema in the reference, so their fields are sampled from one live record.
func (c *Connector) ListObjectMetadata(
	ctx context.Context, objectNames []string,
) (*common.ListObjectMetadataResult, error) {
	selection, err := metadata.Catalog.Select(objectNames)
	if err != nil {
		return nil, err
	}

	result := selection.Result()

	if _, requested := result.Errors[objectNameSubscriptions]; !requested {
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

var errNoRecordToSample = errors.New("no record to sample fields from")

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
		return nil, fmt.Errorf("%w: %w for %s", common.ErrMissingExpectedValues, errNoRecordToSample, objectName)
	}

	record, err := jsonquery.Convertor.ObjectToMap(records[0])
	if err != nil {
		return nil, err
	}

	objectMetadata := common.NewObjectMetadata(displayName(objectName), common.FieldsMetadata{})

	for field, value := range record {
		objectMetadata.AddFieldMetadata(field, common.FieldMetadata{
			DisplayName: displayName(field),
			ValueType:   common.InferValueTypeFromData(value),
		})
	}

	return objectMetadata, nil
}

// displayName turns a snake_case key into words: `created_at` becomes `Created At`.
func displayName(key string) string {
	return naming.CapitalizeFirstLetterEveryWord(strings.ReplaceAll(key, "_", " "))
}
