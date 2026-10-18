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

// ListObjectMetadata describes the built-in objects from GET /schemas, which returns every object
// schema of the workspace in one (cursor-paginated) list, so it is fetched once per call.
// Users are not an object of /objects/{object}/resources; their fields are the attributes
// GET /users returns.
// https://developer.clarify.ai/docs/api-reference/schemas/getSchemas
func (c *Connector) ListObjectMetadata(
	ctx context.Context, objectNames []string,
) (*common.ListObjectMetadataResult, error) {
	if len(objectNames) == 0 {
		return nil, common.ErrMissingObjects
	}

	result := common.NewListObjectMetadataResult()

	var (
		catalog    *schemaCatalog
		catalogErr error
	)

	for _, objectName := range datautils.NewSetFromList(objectNames).List() {
		switch {
		case objectName == objectNameUser:
			result.Result[objectName] = userMetadata()
		case entityObjects.Has(objectName):
			if catalog == nil && catalogErr == nil {
				catalog, catalogErr = c.fetchSchemas(ctx)
			}

			if catalogErr != nil {
				result.AppendError(objectName, catalogErr)

				continue
			}

			metadata, err := catalog.objectMetadata(objectName)
			if err != nil {
				result.AppendError(objectName, err)

				continue
			}

			result.Result[objectName] = *metadata
		default:
			result.AppendError(objectName, common.ErrObjectNotSupported)
		}
	}

	return result, nil
}

// fetchSchemas reads every schema of the workspace, following links.next.
func (c *Connector) fetchSchemas(ctx context.Context) (*schemaCatalog, error) {
	schemasURL, err := c.workspaceURL("schemas")
	if err != nil {
		return nil, err
	}

	catalog := &schemaCatalog{
		byID:     make(map[string]schemaAttributes),
		entities: make(map[string]string),
	}

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
			catalog.byID[resource.ID] = resource.Attributes

			if objectName, ok := entityName(resource.ID); ok {
				catalog.entities[objectName] = resource.ID
			}
		}

		next = ""
		if page.Links.Next != nil {
			next = *page.Links.Next
		}
	}

	return catalog, nil
}

// schemaCatalog holds the schemas of one GET /schemas listing.
type schemaCatalog struct {
	// byID is every schema keyed by its id, the JSON Schema `$id` that `$ref` points to.
	byID map[string]schemaAttributes
	// entities maps an object name to its schema id.
	entities map[string]string
}

// Built-in and custom objects are identified as `.../entities/{object}`; `.../core/*` entries are
// the shared base definitions those schemas build on.
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

func (c schemaCatalog) objectMetadata(objectName string) (*common.ObjectMetadata, error) {
	schemaID, ok := c.entities[objectName]
	if !ok {
		return nil, fmt.Errorf("%w: no schema for %s", common.ErrMissingExpectedValues, objectName)
	}

	schema := c.byID[schemaID]

	fields := common.FieldsMetadata{
		"id": {
			DisplayName:  "ID",
			ValueType:    common.ValueTypeString,
			ProviderType: "string",
			ReadOnly:     new(true),
		},
	}

	for key, property := range c.properties(schemaID, datautils.NewSet[string]()) {
		if !property.inAttributes() {
			continue
		}

		fields[key] = property.fieldMetadata(key)
	}

	return common.NewObjectMetadata(schema.displayName(objectName), fields), nil
}

// properties returns a schema's properties together with those of the schemas it composes
// through `$ref` and `allOf`, the core definitions that entity schemas build on.
// The schema's own properties win over inherited ones.
func (c schemaCatalog) properties(schemaID string, visited datautils.Set[string]) map[string]schemaProperty {
	if visited.Has(schemaID) {
		return nil
	}

	visited.AddOne(schemaID)

	schema, ok := c.byID[schemaID]
	if !ok {
		return nil
	}

	result := make(map[string]schemaProperty)

	for _, reference := range schema.references() {
		maps.Copy(result, c.properties(reference, visited))
	}

	maps.Copy(result, schema.Properties)

	return result
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
	Ref        string                    `json:"$ref"`
	AllOf      []schemaReference         `json:"allOf"`
}

type schemaReference struct {
	Ref string `json:"$ref"`
}

type schemaLabel struct {
	Singular string `json:"singular"`
	Plural   string `json:"plural"`
}

func (s schemaAttributes) references() []string {
	references := make([]string, 0, len(s.AllOf)+1)

	if s.Ref != "" {
		references = append(references, s.Ref)
	}

	for _, item := range s.AllOf {
		if item.Ref != "" {
			references = append(references, item.Ref)
		}
	}

	return references
}

func (s schemaAttributes) displayName(objectName string) string {
	switch {
	case s.Label.Plural != "":
		return s.Label.Plural
	case s.Title != "":
		return s.Title
	default:
		return objectName
	}
}

type schemaProperty struct {
	Type         jsonSchemaType      `json:"type"`
	Format       string              `json:"format"`
	Title        string              `json:"title"`
	Enum         []any               `json:"enum"`
	Items        *schemaProperty     `json:"items"`
	OneOf        []schemaReference   `json:"oneOf"`
	ReadOnly     *bool               `json:"readOnly"`
	Relationship *schemaRelationship `json:"xClarifyRelationship"`
}

// schemaRelationship links a field to another object:
// {"kind": "many-to-one", "entity": "company", "field": "projects"}.
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

// inAttributes tells whether Read returns the property among a resource's attributes.
// The to-one side of a relationship is a foreign-key attribute holding the related record's ID
// (person `company_id`). The to-many side (a company's people, a person's deals) is not an
// attribute: it is only returned under `relationships` when requested with `include`, which this
// connector does not send.
func (p schemaProperty) inAttributes() bool {
	if p.Relationship == nil {
		return true
	}

	return p.Relationship.Kind == "many-to-one" || p.Relationship.Kind == "one-to-one"
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

	if p.Relationship != nil && entityObjects.Has(p.Relationship.Entity) {
		metadata.ReferenceTo = []string{p.Relationship.Entity}
	}

	return metadata
}

func (p schemaProperty) providerType() string {
	if p.Relationship != nil {
		return p.Relationship.Kind
	}

	// A property composed from a core definition is named after it:
	// {"oneOf": [{"$ref": "https://getclarify.ai/schemas/core/agentInstructions"}]} -> agentInstructions.
	if len(p.Type) == 0 && len(p.OneOf) == 1 {
		return p.OneOf[0].Ref[strings.LastIndex(p.OneOf[0].Ref, "/")+1:]
	}

	providerType := p.Type.String()
	if p.Format != "" {
		providerType += ":" + p.Format
	}

	return providerType
}

// valueType maps a JSON Schema property to an Ampersand value type.
// Properties defined by `oneOf`/`$ref` (names, collections such as email_addresses, locations)
// carry no `type` and are objects in records, so they map to other.
func (p schemaProperty) valueType() (common.ValueType, []common.FieldValue) {
	if p.Relationship != nil {
		return common.ValueTypeReference, nil
	}

	if values := enumValues(p.Enum); len(values) != 0 {
		return common.ValueTypeSingleSelect, values
	}

	if len(p.Type) != 1 {
		return common.ValueTypeOther, nil
	}

	if p.Type[0] == "array" && p.Items != nil {
		if values := enumValues(p.Items.Enum); len(values) != 0 {
			return common.ValueTypeMultiSelect, values
		}
	}

	return scalarValueType(p.Type[0], p.Format), nil
}

// scalarValueType types a JSON Schema type; string formats such as uuid, email, uri or markdown
// stay strings. Arrays without enum items and objects are other.
func scalarValueType(jsonType, format string) common.ValueType {
	switch jsonType {
	case "string":
		switch format {
		case "date-time":
			return common.ValueTypeDateTime
		case "date":
			return common.ValueTypeDate
		default:
			return common.ValueTypeString
		}
	case "number":
		return common.ValueTypeFloat
	case "integer":
		return common.ValueTypeInt
	case "boolean":
		return common.ValueTypeBoolean
	default:
		return common.ValueTypeOther
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

// userMetadata lists the user attributes documented for GET /users, plus `is_service_account`,
// which the live API also returns. `name` holds first_name, last_name and full_name;
// `roles` is a collection ({"items": ["admin"]}).
// https://developer.clarify.ai/docs/api-reference/users/getUsers
func userMetadata() common.ObjectMetadata {
	return *common.NewObjectMetadata("Users", common.FieldsMetadata{
		"id":                 userField("ID", common.ValueTypeString, "string"),
		"_id":                userField("Record ID", common.ValueTypeString, "string"),
		"name":               userField("Name", common.ValueTypeOther, "object"),
		"roles":              userField("Roles", common.ValueTypeOther, "object"),
		"email":              userField("Email", common.ValueTypeString, "string"),
		"profile_picture":    userField("Profile Picture", common.ValueTypeString, "string"),
		"is_service_account": userField("Is Service Account", common.ValueTypeBoolean, "boolean"),
		"last_active_at":     userField("Last Active At", common.ValueTypeDateTime, "string:date-time"),
		"_created_at":        userField("Created At", common.ValueTypeDateTime, "string:date-time"),
		"_updated_at":        userField("Updated At", common.ValueTypeDateTime, "string:date-time"),
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
