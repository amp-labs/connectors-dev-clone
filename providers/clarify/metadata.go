package clarify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/naming"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/datautils"
)

// Schema ids end with the entity name, e.g. https://getclarify.ai/schemas/entities/person.
// https://developer.clarify.ai/docs/guides/schemas-and-relationships#schema-fields
const entitySchemaMarker = "/entities/"

// ListObjectMetadata describes objects from GET /v1/workspaces/{workspace}/schemas, which returns the
// JSON Schema of every object in the workspace, custom fields included. One call serves all objects.
// https://developer.clarify.ai/docs/api-reference/schemas/getSchemas
func (c *Connector) ListObjectMetadata(
	ctx context.Context, objectNames []string,
) (*common.ListObjectMetadataResult, error) {
	if len(objectNames) == 0 {
		return nil, common.ErrMissingObjects
	}

	result := common.NewListObjectMetadataResult()

	schemas, err := c.fetchSchemas(ctx)
	if err != nil {
		return nil, err
	}

	for _, objectName := range datautils.NewSetFromList(objectNames).List() {
		if !datautils.NewSetFromList(supportedObjects).Has(objectName) {
			result.AppendError(objectName, common.ErrObjectNotSupported)

			continue
		}

		schema, ok := schemas[schemaEntityName(objectName)]
		if !ok {
			result.AppendError(objectName, fmt.Errorf("%w: no schema for %s", common.ErrMissingExpectedValues, objectName))

			continue
		}

		result.Result[objectName] = *schema.objectMetadata(objectName)
	}

	return result, nil
}

func schemaEntityName(objectName string) string {
	if objectName == objectNameUsers {
		return "user"
	}

	return objectName
}

// fetchSchemas reads every page of schemas, keyed by entity name.
func (c *Connector) fetchSchemas(ctx context.Context) (map[string]entitySchema, error) {
	schemasURL, err := urlbuilder.New(c.ProviderInfo().BaseURL, apiVersion, "workspaces", c.workspace, "schemas")
	if err != nil {
		return nil, err
	}

	schemas := make(map[string]entitySchema)
	next := schemasURL.String()

	for next != "" {
		response, err := c.JSONHTTPClient().Get(ctx, next)
		if err != nil {
			return nil, err
		}

		page, err := common.UnmarshalJSON[schemasResponse](response)
		if err != nil {
			return nil, err
		}

		if page == nil {
			return nil, common.ErrEmptyJSONHTTPResponse
		}

		for _, resource := range page.Data {
			if _, entity, ok := strings.Cut(resource.ID, entitySchemaMarker); ok {
				schemas[entity] = resource.Attributes
			}
		}

		next = ""
		if page.Links.Next != nil {
			next = *page.Links.Next
		}
	}

	return schemas, nil
}

type schemasResponse struct {
	Data []struct {
		ID         string       `json:"id"`
		Attributes entitySchema `json:"attributes"`
	} `json:"data"`
	Links struct {
		Next *string `json:"next"`
	} `json:"links"`
}

type entitySchema struct {
	Title         string `json:"title"`
	XClarifyLabel struct {
		Singular string `json:"singular"`
		Plural   string `json:"plural"`
	} `json:"xClarifyLabel"`
	Properties map[string]propertySchema `json:"properties"`
}

type propertySchema struct {
	Type                 schemaType          `json:"type"`
	Format               string              `json:"format"`
	Title                string              `json:"title"`
	Enum                 []any               `json:"enum"`
	Items                *propertySchema     `json:"items"`
	ReadOnly             *bool               `json:"readOnly"`
	XClarifyRelationship *relationshipSchema `json:"xClarifyRelationship"`
}

type relationshipSchema struct {
	Kind   string `json:"kind"`
	Entity string `json:"entity"`
	Field  string `json:"field"`
}

// schemaType is a JSON Schema `type`: a single name or a list such as ["string", "null"].
type schemaType []string

func (t *schemaType) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*t = schemaType{single}

		return nil
	}

	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	*t = list

	return nil
}

// primary is the type without "null".
func (t schemaType) primary() string {
	for _, name := range t {
		if name != "null" {
			return name
		}
	}

	return ""
}

func (s entitySchema) objectMetadata(objectName string) *common.ObjectMetadata {
	displayName := s.XClarifyLabel.Plural
	if displayName == "" {
		displayName = naming.CapitalizeFirstLetterEveryWord(objectName)
	}

	metadata := common.NewObjectMetadata(displayName, common.FieldsMetadata{})

	// Read adds the JSON:API resource id to every record.
	metadata.AddFieldMetadata("id", common.FieldMetadata{
		DisplayName:  "ID",
		ValueType:    common.ValueTypeString,
		ProviderType: "string",
		ReadOnly:     new(true),
	})

	for name, property := range s.Properties {
		metadata.AddFieldMetadata(name, property.fieldMetadata(name))
	}

	return metadata
}

func (p propertySchema) fieldMetadata(name string) common.FieldMetadata {
	displayName := p.Title
	if displayName == "" {
		displayName = name
	}

	valueType, values := p.valueType()

	field := common.FieldMetadata{
		DisplayName:  displayName,
		ValueType:    valueType,
		ProviderType: p.providerType(),
		ReadOnly:     p.ReadOnly,
		Values:       values,
	}

	if valueType == common.ValueTypeReference {
		field.ReferenceTo = []string{p.XClarifyRelationship.Entity}
	}

	return field
}

func (p propertySchema) providerType() string {
	providerType := p.Type.primary()
	if p.Format != "" {
		providerType += "/" + p.Format
	}

	return providerType
}

// valueType maps a JSON Schema property to an Ampersand value type.
// A many-to-one relationship holds one record id (e.g. person.company_id); other relationship
// kinds hold several and stay other.
func (p propertySchema) valueType() (common.ValueType, []common.FieldValue) {
	if p.XClarifyRelationship != nil {
		if p.XClarifyRelationship.Kind == "many-to-one" && p.XClarifyRelationship.Entity != "" {
			return common.ValueTypeReference, nil
		}

		return common.ValueTypeOther, nil
	}

	if len(p.Enum) != 0 {
		return common.ValueTypeSingleSelect, enumValues(p.Enum)
	}

	switch p.Type.primary() {
	case "string":
		switch p.Format {
		case "date":
			return common.ValueTypeDate, nil
		case "date-time":
			return common.ValueTypeDateTime, nil
		default:
			return common.ValueTypeString, nil
		}
	case "integer":
		return common.ValueTypeInt, nil
	case "number":
		return common.ValueTypeFloat, nil
	case "boolean":
		return common.ValueTypeBoolean, nil
	case "array":
		if p.Items != nil && len(p.Items.Enum) != 0 {
			return common.ValueTypeMultiSelect, enumValues(p.Items.Enum)
		}

		return common.ValueTypeOther, nil
	default:
		return common.ValueTypeOther, nil
	}
}

func enumValues(enum []any) []common.FieldValue {
	values := make([]common.FieldValue, 0, len(enum))

	for _, value := range enum {
		text, ok := value.(string)
		if !ok {
			continue
		}

		values = append(values, common.FieldValue{Value: text, DisplayValue: text})
	}

	return values
}
