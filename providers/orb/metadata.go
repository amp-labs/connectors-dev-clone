package orb

import (
	"context"
	"net/http"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/naming"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/jsonquery"
)

// Records sampled per object. Orb serialises every field of a record, null when unset, so one
// record names all fields; later records only give a type to fields that were null in earlier ones.
const metadataSampleSize = "20"

// buildObjectMetadataRequest samples the object's list endpoint, the one Read uses.
// Metadata is sampled because the OpenAPI spec the docs cite (/api-reference/orb-openapi.json)
// is not available to this build, and the docs pages carry no schema for subscriptions.
func (c *Connector) buildObjectMetadataRequest(ctx context.Context, objectName string) (*http.Request, error) {
	if !supportedObjectSet.Has(objectName) {
		return nil, common.ErrObjectNotSupported
	}

	metadataURL, err := urlbuilder.New(c.ProviderInfo().BaseURL, apiVersion, objectName)
	if err != nil {
		return nil, err
	}

	metadataURL.WithQueryParam("limit", metadataSampleSize)

	return http.NewRequestWithContext(ctx, http.MethodGet, metadataURL.String(), nil)
}

func (c *Connector) parseObjectMetadataResponse(
	ctx context.Context,
	objectName string,
	request *http.Request,
	response *common.JSONHTTPResponse,
) (*common.ObjectMetadata, error) {
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

	valueTypes := make(map[string]common.ValueType)

	for _, node := range records {
		record, err := jsonquery.Convertor.ObjectToMap(node)
		if err != nil {
			return nil, err
		}

		for field, value := range record {
			if valueTypes[field] == "" || valueTypes[field] == common.ValueTypeOther {
				valueTypes[field] = common.InferValueTypeFromData(value)
			}
		}
	}

	fields := make(common.FieldsMetadata, len(valueTypes))

	for field, valueType := range valueTypes {
		fields[field] = common.FieldMetadata{
			DisplayName: displayName(field),
			ValueType:   valueType,
		}
	}

	return common.NewObjectMetadata(displayName(objectName), fields), nil
}

func displayName(name string) string {
	return naming.CapitalizeFirstLetterEveryWord(naming.SeparateUnderscoreWords(name))
}
