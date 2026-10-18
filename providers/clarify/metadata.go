package clarify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/datautils"
)

// ListObjectMetadata describes the built-in objects from GET /schemas, which returns every object
// schema of the workspace in one (cursor-paginated) list, so it is fetched once per call.
// Users are not an object of /objects/{object}/resources; their fields are the documented
// attributes of GET /users.
// https://developer.clarify.ai/docs/api-reference/schemas/getSchemas
func (c *Connector) ListObjectMetadata(
	ctx context.Context, objectNames []string,
) (*common.ListObjectMetadataResult, error) {
	if len(objectNames) == 0 {
		return nil, common.ErrMissingObjects
	}

	result := common.NewListObjectMetadataResult()

	var (
		schemas    map[string]schemaAttributes
		schemasErr error
	)

	for _, objectName := range datautils.NewSetFromList(objectNames).List() {
		switch {
		case objectName == objectNameUser:
			result.Result[objectName] = userMetadata()
		case entityObjects.Has(objectName):
			if schemas == nil && schemasErr == nil {
				schemas, schemasErr = c.fetchSchemas(ctx)
			}

			if schemasErr != nil {
				result.AppendError(objectName, schemasErr)

				continue
			}

			schema, ok := schemas[objectName]
			if !ok {
				result.AppendError(objectName,
					fmt.Errorf("%w: no schema for %s", common.ErrMissingExpectedValues, objectName))

				continue
			}

			result.Result[objectName] = schema.objectMetadata(objectName)
		default:
			result.AppendError(objectName, common.ErrObjectNotSupported)
		}
	}

	return result, nil
}

// fetchSchemas returns the entity schemas keyed by object name, following links.next.
func (c *Connector) fetchSchemas(ctx context.Context) (map[string]schemaAttributes, error) {
	schemasURL, err := c.workspaceURL("schemas")
	if err != nil {
		return nil, err
	}

	schemas := make(map[string]schemaAttributes)

	for next := schemasURL.String(); next != ""; {
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
			if objectName, ok := entityName(resource.ID); ok {
				schemas[objectName] = resource.Attributes
			}
		}

		next = ""
		if page.Links.Next != nil {
			next = *page.Links.Next
		}
	}

	return schemas, nil
}

// Built-in and custom objects are identified as `.../entities/{object}`; `.../core/*` entries are
// the shared base definitions those build on.
// https://developer.clarify.ai/docs/guides/schemas-and-relationships#schema-fields
const entitySchemaMarker = "/entities/"

func entityName(schemaID string) (string, bool) {
	index := strings.LastIndex(schemaID, entitySchemaMarker)
	if index < 0 {
		return "", false
	}

	name := schemaID[index+len(entitySchemaMarker):]

	return name, name != ""
}

type schemasResponse struct {
	Data  []schemaResource `json:"data"`
	Links struct {
		Next *string `json:"next"`
	} `json:"links"`
}

type schemaResource struct {
	ID         string           `json:"id"`
	Attributes schemaAttributes `json:"attributes"`
}

type schemaAttributes struct {
	Title      string                    `json:"title"`
	Label      schemaLabel               `json:"xClarifyLabel"`
	Properties map[string]schemaProperty `json:"properties"`
}

type schemaLabel struct {
	Singular string `json:"singular"`
	Plural   string `json:"plural"`
}

type schemaProperty struct {
	Type         jsonSchemaType      `json:"type"`
	Format       string              `json:"format"`
	Title        string              `json:"title"`
	Enum         []any               `json:"enum"`
	Items        *schemaProperty     `json:"items"`
	ReadOnly     *bool               `json:"readOnly"`
	Relationship *schemaRelationship `json:"xClarifyRelationship"`
}

type schemaRelationship struct {
	Kind   string `json:"kind"`
	Entity string `json:"entity"`
	Field  string `json:"field"`
}

// jsonSchemaType is a JSON Schema `type`: a string or a list such as ["string", "null"].
// The null alternative only marks the field as nullable and is dropped.
type jsonSchemaType []string

func (t *jsonSchemaType) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*t = jsonSchemaType{single}

		return nil
	}

	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	types := make(jsonSchemaType, 0, len(list))

	for _, item := range list {
		if item != "null" {
			types = append(types, item)
		}
	}

	*t = types

	return nil
}

func (t jsonSchemaType) String() string {
	return strings.Join(t, ",")
}

func (s schemaAttributes) objectMetadata(objectName string) common.ObjectMetadata {
	displayName := s.Label.Plural
	if displayName == "" {
		displayName = s.Title
	}

	if displayName == "" {
		displayName = objectName
	}

	fields := common.FieldsMetadata{
		"id": {
			DisplayName:  "ID",
			ValueType:    common.ValueTypeString,
			ProviderType: "string",
			ReadOnly:     new(true),
		},
	}

	for key, property := range s.Properties {
		fields[key] = property.fieldMetadata(key)
	}

	return *common.NewObjectMetadata(displayName, fields)
}

func (p schemaProperty) fieldMetadata(key string) common.FieldMetadata {
	displayName := p.Title
	if displayName == "" {
		displayName = key
	}

	valueType, values := p.valueType()

	metadata := common.FieldMetadata{
		DisplayName:  displayName,
		ValueType:    valueType,
		ProviderType: p.providerType(),
		ReadOnly:     p.ReadOnly,
		Values:       values,
	}

	if valueType == common.ValueTypeReference && entityObjects.Has(p.Relationship.Entity) {
		metadata.ReferenceTo = []string{p.Relationship.Entity}
	}

	return metadata
}

func (p schemaProperty) providerType() string {
	providerType := p.Type.String()
	if p.Format != "" {
		providerType += ":" + p.Format
	}

	return providerType
}

// valueType maps a JSON Schema property to an Ampersand value type.
// A many-to-one or one-to-one relationship holds the related record's ID (`company_id`).
func (p schemaProperty) valueType() (common.ValueType, []common.FieldValue) {
	if p.Relationship != nil {
		switch p.Relationship.Kind {
		case "many-to-one", "one-to-one":
			return common.ValueTypeReference, nil
		default:
			return common.ValueTypeOther, nil
		}
	}

	if values := enumValues(p.Enum); len(values) != 0 {
		return common.ValueTypeSingleSelect, values
	}

	if len(p.Type) != 1 {
		return common.ValueTypeOther, nil
	}

	switch p.Type[0] {
	case "string":
		switch p.Format {
		case "date-time":
			return common.ValueTypeDateTime, nil
		case "date":
			return common.ValueTypeDate, nil
		default:
			return common.ValueTypeString, nil
		}
	case "number":
		return common.ValueTypeFloat, nil
	case "integer":
		return common.ValueTypeInt, nil
	case "boolean":
		return common.ValueTypeBoolean, nil
	case "array":
		if p.Items != nil {
			if values := enumValues(p.Items.Enum); len(values) != 0 {
				return common.ValueTypeMultiSelect, values
			}
		}

		return common.ValueTypeOther, nil
	default:
		return common.ValueTypeOther, nil
	}
}

func enumValues(enum []any) []common.FieldValue {
	values := make([]common.FieldValue, 0, len(enum))

	for _, item := range enum {
		text, ok := item.(string)
		if !ok {
			continue
		}

		values = append(values, common.FieldValue{Value: text, DisplayValue: text})
	}

	return values
}

// userMetadata lists the user attributes documented for GET /users and GET /users/{userId}.
// https://developer.clarify.ai/docs/api-reference/users/getUsers
func userMetadata() common.ObjectMetadata {
	return *common.NewObjectMetadata("Users", common.FieldsMetadata{
		"id":              userField("ID", common.ValueTypeString, "string"),
		"_id":             userField("ID", common.ValueTypeString, "string"),
		"name":            userField("Name", common.ValueTypeOther, "object"),
		"roles":           userField("Roles", common.ValueTypeOther, "object"),
		"email":           userField("Email", common.ValueTypeString, "string"),
		"profile_picture": userField("Profile Picture", common.ValueTypeString, "string"),
		"last_active_at":  userField("Last Active At", common.ValueTypeDateTime, "string:date-time"),
		"_created_at":     userField("Created At", common.ValueTypeDateTime, "string:date-time"),
		"_updated_at":     userField("Updated At", common.ValueTypeDateTime, "string:date-time"),
	})
}

func userField(displayName string, valueType common.ValueType, providerType string) common.FieldMetadata {
	return common.FieldMetadata{
		DisplayName:  displayName,
		ValueType:    valueType,
		ProviderType: providerType,
		ReadOnly:     new(true),
	}
}
