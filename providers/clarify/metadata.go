package clarify

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/naming"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/datautils"
)

// Object schemas have ids ending in `/entities/{object}`, e.g. https://getclarify.ai/schemas/entities/person.
// The `/core/*` entries are base definitions those schemas build on.
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

	schemas, err := c.fetchSchemas(ctx)
	if err != nil {
		return nil, err
	}

	result := common.NewListObjectMetadataResult()

	for _, objectName := range datautils.NewSetFromList(objectNames).List() {
		if !slices.Contains(supportedObjects, objectName) {
			result.AppendError(objectName, common.ErrObjectNotSupported)

			continue
		}

		schema, ok := schemas[objectName]
		if !ok {
			result.AppendError(objectName,
				fmt.Errorf("%w: schema for %s", common.ErrMissingExpectedValues, objectName))

			continue
		}

		result.Result[objectName] = *schema.objectMetadata(objectName)
	}

	return result, nil
}

// fetchSchemas reads every page of schemas, keyed by object name.
func (c *Connector) fetchSchemas(ctx context.Context) (map[string]entitySchema, error) {
	schemasURL, err := urlbuilder.New(c.ProviderInfo().BaseURL, apiVersion, "workspaces", c.workspace, "schemas")
	if err != nil {
		return nil, err
	}

	requestURL, err := schemasURL.ToURL()
	if err != nil {
		return nil, err
	}

	schemas := make(map[string]entitySchema)
	nextPage := makeNextPageFunc(requestURL)
	next := requestURL.String()

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
			if _, objectName, ok := strings.Cut(resource.ID, entitySchemaMarker); ok {
				schemas[objectName] = resource.Attributes
			}
		}

		body, ok := response.Body()
		if !ok {
			return nil, common.ErrEmptyJSONHTTPResponse
		}

		next, err = nextPage(body)
		if err != nil {
			return nil, err
		}
	}

	return schemas, nil
}

type schemasResponse struct {
	Data []struct {
		ID         string       `json:"id"`
		Attributes entitySchema `json:"attributes"`
	} `json:"data"`
}

type entitySchema struct {
	XClarifyLabel struct {
		Plural string `json:"plural"`
	} `json:"xClarifyLabel"`
	Properties map[string]propertySchema `json:"properties"`
}

type propertySchema struct {
	Type                 schemaType                `json:"type"`
	Format               string                    `json:"format"`
	Title                string                    `json:"title"`
	Enum                 []any                     `json:"enum"`
	Ref                  string                    `json:"$ref"`
	OneOf                []propertySchema          `json:"oneOf"`
	Items                *propertySchema           `json:"items"`
	Properties           map[string]propertySchema `json:"properties"`
	ReadOnly             *bool                     `json:"readOnly"`
	XClarifyRelationship *relationshipSchema       `json:"xClarifyRelationship"`
}

type relationshipSchema struct {
	Kind   string `json:"kind"`
	Entity string `json:"entity"`
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

// primary is the first type other than "null".
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

	// flattenRecord adds the JSON:API resource id to every record.
	metadata.AddFieldMetadata("id", common.FieldMetadata{
		DisplayName:  "ID",
		ValueType:    common.ValueTypeString,
		ProviderType: "string",
		ReadOnly:     new(true),
	})

	for name, property := range s.Properties {
		if !property.isAttribute() {
			continue
		}

		metadata.AddFieldMetadata(name, property.fieldMetadata(name))
	}

	return metadata
}

// isAttribute reports whether the property comes back in a resource's attributes.
// One-to-many and many-to-many relationships (person.deals, company.people) are collections of ids that
// the list endpoint returns only under `relationships` when requested with `include`, so they are left out.
func (p propertySchema) isAttribute() bool {
	if p.XClarifyRelationship == nil {
		return true
	}

	kind := p.XClarifyRelationship.Kind

	return kind == "many-to-one" || kind == "one-to-one"
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

// providerType is the schema's format (date-time, uuid, markdown...), else its type, else the core
// definition it references (personName, collectionOfEmails...).
func (p propertySchema) providerType() string {
	if p.Format != "" {
		return p.Format
	}

	if primary := p.Type.primary(); primary != "" {
		return primary
	}

	for _, option := range p.OneOf {
		if option.Ref != "" {
			return path.Base(option.Ref)
		}
	}

	return ""
}

// valueType maps a schema property to an Ampersand value type.
// A many-to-one or one-to-one relationship holds a single record id; it is a reference when it
// points to an object this connector reads, and a string id otherwise (e.g. meeting.public_link_id).
// Collection fields are objects shaped {"items": [...]}; those whose items are enum values
// (person.labels, user.roles) are multi-selects.
func (p propertySchema) valueType() (common.ValueType, []common.FieldValue) {
	if p.XClarifyRelationship != nil {
		if slices.Contains(supportedObjects, p.XClarifyRelationship.Entity) {
			return common.ValueTypeReference, nil
		}

		return common.ValueTypeString, nil
	}

	switch p.Format {
	case "date-time":
		return common.ValueTypeDateTime, nil
	case "date":
		return common.ValueTypeDate, nil
	}

	if values := enumValues(p.Enum); len(values) != 0 {
		return common.ValueTypeSingleSelect, values
	}

	if items, ok := p.Properties["items"]; ok && items.Items != nil {
		if values := enumValues(items.Items.Enum); len(values) != 0 {
			return common.ValueTypeMultiSelect, values
		}
	}

	switch p.Type.primary() {
	case "string":
		return common.ValueTypeString, nil
	case "integer":
		return common.ValueTypeInt, nil
	case "number":
		return common.ValueTypeFloat, nil
	case "boolean":
		return common.ValueTypeBoolean, nil
	default:
		return common.ValueTypeOther, nil
	}
}

// enumValues lists the options of an enum; null, which marks the field as clearable, is not an option.
func enumValues(enum []any) []common.FieldValue {
	values := make([]common.FieldValue, 0, len(enum))

	for _, value := range enum {
		if text, ok := value.(string); ok {
			values = append(values, common.FieldValue{Value: text, DisplayValue: text})
		}
	}

	return values
}
