package clarify

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/datautils"
)

// Built-in and custom objects' schemas are identified by `.../entities/{object}`; `.../core/*`
// entries are shared base definitions, not objects.
// https://developer.clarify.ai/docs/guides/schemas-and-relationships#schema-fields
const entitySchemaMarker = "/entities/"

// listObjectMetadata describes person, company, deal, meeting and task from the workspace's
// JSON Schemas (GET /v1/workspaces/{workspace}/schemas, one call for every object), so custom
// fields and select options are included. Users have no entity schema documented; their fields
// are the ones the users endpoint documents.
// https://developer.clarify.ai/docs/api-reference/schemas/getSchemas
func (c *Connector) listObjectMetadata(
	ctx context.Context, objectNames []string,
) (*common.ListObjectMetadataResult, error) {
	result := common.NewListObjectMetadataResult()
	requested := datautils.NewSetFromList(objectNames)

	if requested.Has(objectNameUser) {
		result.Result[objectNameUser] = *common.NewObjectMetadata("Users", userFields())
	}

	var (
		schemas   map[string]entitySchema
		schemaErr error
	)

	for _, objectName := range requested.List() {
		if objectName == objectNameUser {
			continue
		}

		if !entityObjects.Has(objectName) {
			result.AppendError(objectName, common.ErrObjectNotSupported)

			continue
		}

		if schemas == nil && schemaErr == nil {
			schemas, schemaErr = c.fetchSchemas(ctx)
		}

		if schemaErr != nil {
			result.AppendError(objectName, schemaErr)

			continue
		}

		schema, ok := schemas[objectName]
		if !ok {
			result.AppendError(objectName,
				fmt.Errorf("%w: no schema for object %s", common.ErrMissingExpectedValues, objectName))

			continue
		}

		result.Result[objectName] = *schema.objectMetadata(objectName)
	}

	return result, nil
}

// fetchSchemas reads every schema page and indexes the entity schemas by object name.
func (c *Connector) fetchSchemas(ctx context.Context) (map[string]entitySchema, error) {
	schemasURL, err := c.workspaceURL("schemas")
	if err != nil {
		return nil, err
	}

	schemas := make(map[string]entitySchema)

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
			if objectName, ok := entityName(resource.Attributes.ID); ok {
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

func entityName(schemaID string) (string, bool) {
	index := strings.LastIndex(schemaID, entitySchemaMarker)
	if index < 0 {
		return "", false
	}

	return schemaID[index+len(entitySchemaMarker):], true
}

type schemasResponse struct {
	Data []struct {
		Attributes entitySchema `json:"attributes"`
	} `json:"data"`
	Links struct {
		Next *string `json:"next"`
	} `json:"links"`
}

type entitySchema struct {
	ID         string                    `json:"$id"`
	Title      string                    `json:"title"`
	Label      *schemaLabel              `json:"xClarifyLabel"` // nolint:tagliatelle
	Properties map[string]schemaProperty `json:"properties"`
}

type schemaLabel struct {
	Singular string `json:"singular"`
	Plural   string `json:"plural"`
}

type schemaProperty struct {
	Type         schemaType          `json:"type"`
	Format       string              `json:"format"`
	Title        string              `json:"title"`
	Enum         []any               `json:"enum"`
	Items        *schemaProperty     `json:"items"`
	ReadOnly     *bool               `json:"readOnly"`
	Relationship *schemaRelationship `json:"xClarifyRelationship"` // nolint:tagliatelle
}

type schemaRelationship struct {
	Kind   string `json:"kind"`
	Entity string `json:"entity"`
}

// schemaType is a JSON Schema `type`: a single name, or a list such as ["string", "null"].
// It holds the names other than "null".
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

	types := make(schemaType, 0, len(list))

	for _, name := range list {
		if name != "null" {
			types = append(types, name)
		}
	}

	*t = types

	return nil
}

func (s entitySchema) objectMetadata(objectName string) *common.ObjectMetadata {
	displayName := s.Title
	if s.Label != nil && s.Label.Plural != "" {
		displayName = s.Label.Plural
	}

	if displayName == "" {
		displayName = objectName
	}

	fields := make(common.FieldsMetadata, len(s.Properties)+len(recordSystemFields)+1)

	// Every record carries these, whether or not the entity schema lists them.
	// https://developer.clarify.ai/docs/guides/schemas-and-relationships#user-identifiers
	maps.Copy(fields, recordSystemFields)

	for name, property := range s.Properties {
		fields[name] = property.fieldMetadata(name)
	}

	// The resource id, which Read adds to the attributes.
	fields["id"] = idField

	return common.NewObjectMetadata(displayName, fields)
}

func (p schemaProperty) fieldMetadata(name string) common.FieldMetadata {
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

	if p.Relationship != nil && entityObjects.Has(p.Relationship.Entity) {
		field.ReferenceTo = []string{p.Relationship.Entity}
	}

	return field
}

// valueType maps a JSON Schema property to an Ampersand type. A many-to-one or one-to-one
// relationship holds one record id; the to-many sides hold `{"items": [...]}` collections.
// https://developer.clarify.ai/docs/api-reference/schemas/createSchemaRelationshipProperties
func (p schemaProperty) valueType() (common.ValueType, []common.FieldValue) { // nolint:cyclop
	if p.Relationship != nil {
		if p.Relationship.Kind == "many-to-one" || p.Relationship.Kind == "one-to-one" {
			return common.ValueTypeReference, nil
		}

		return common.ValueTypeOther, nil
	}

	if values := enumValues(p.Enum); len(values) != 0 {
		return common.ValueTypeSingleSelect, values
	}

	if p.Items != nil {
		if values := enumValues(p.Items.Enum); len(values) != 0 {
			return common.ValueTypeMultiSelect, values
		}
	}

	if len(p.Type) != 1 {
		return common.ValueTypeOther, nil
	}

	switch p.Type[0] {
	case "string":
		switch p.Format {
		case "date":
			return common.ValueTypeDate, nil
		case "date-time":
			return common.ValueTypeDateTime, nil
		default:
			return common.ValueTypeString, nil
		}
	case "number":
		return common.ValueTypeFloat, nil
	case "integer":
		return common.ValueTypeInt, nil
	case "boolean":
		return common.ValueTypeBoolean, nil
	default:
		return common.ValueTypeOther, nil
	}
}

func (p schemaProperty) providerType() string {
	if p.Format != "" {
		return p.Format
	}

	return strings.Join(p.Type, ",")
}

// Enum values are matched case-sensitively and are their own labels.
func enumValues(enum []any) []common.FieldValue {
	values := make([]common.FieldValue, 0, len(enum))

	for _, value := range enum {
		if text, ok := value.(string); ok {
			values = append(values, common.FieldValue{Value: text, DisplayValue: text})
		}
	}

	return values
}

var idField = common.FieldMetadata{ // nolint:gochecknoglobals
	DisplayName:  "ID",
	ValueType:    common.ValueTypeString,
	ProviderType: "string",
	ReadOnly:     new(true),
}

var recordSystemFields = common.FieldsMetadata{ // nolint:gochecknoglobals
	"_id":         systemField("ID", common.ValueTypeString, "string"),
	"_created_at": systemField("Created At", common.ValueTypeDateTime, "date-time"),
	"_updated_at": systemField("Updated At", common.ValueTypeDateTime, "date-time"),
	"_created_by": systemField("Created By", common.ValueTypeString, "string"),
	"_updated_by": systemField("Updated By", common.ValueTypeString, "string"),
}

// userFields are the attributes of a user resource.
// https://developer.clarify.ai/docs/api-reference/users/getUsers
func userFields() common.FieldsMetadata {
	return common.FieldsMetadata{
		"id":              idField,
		"_id":             recordSystemFields["_id"],
		"name":            userField("Name", common.ValueTypeOther, "object"),
		"roles":           userField("Roles", common.ValueTypeOther, "object"),
		"email":           userField("Email", common.ValueTypeString, "string"),
		"profile_picture": userField("Profile Picture", common.ValueTypeString, "string"),
		"last_active_at":  userField("Last Active At", common.ValueTypeDateTime, "date-time"),
		// Returned by the live API, not in the docs' example.
		"is_service_account": userField("Is Service Account", common.ValueTypeBoolean, "boolean"),
		"_created_at":        recordSystemFields["_created_at"],
		"_updated_at":        recordSystemFields["_updated_at"],
	}
}

// systemField describes a field Clarify manages; `_`-prefixed fields are rejected in writes.
// https://developer.clarify.ai/docs/api-basics/errors#422-unprocessable-entity
func systemField(displayName string, valueType common.ValueType, providerType string) common.FieldMetadata {
	return common.FieldMetadata{
		DisplayName:  displayName,
		ValueType:    valueType,
		ProviderType: providerType,
		ReadOnly:     new(true),
	}
}

func userField(displayName string, valueType common.ValueType, providerType string) common.FieldMetadata {
	return common.FieldMetadata{
		DisplayName:  displayName,
		ValueType:    valueType,
		ProviderType: providerType,
	}
}
