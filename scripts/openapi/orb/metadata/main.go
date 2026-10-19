package main

import (
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/goutils"
	"github.com/amp-labs/connectors/scripts/openapi/internal/api"
	"github.com/amp-labs/connectors/scripts/openapi/internal/api/filters"
	mapping "github.com/amp-labs/connectors/scripts/openapi/internal/api/map"
	"github.com/amp-labs/connectors/scripts/openapi/internal/api/output"
	"github.com/amp-labs/connectors/scripts/openapi/internal/api/pipeline"
	"github.com/amp-labs/connectors/scripts/openapi/internal/api/reducers"
	"github.com/amp-labs/connectors/scripts/openapi/internal/api/spec"
	"github.com/amp-labs/connectors/scripts/openapi/orb/internal/files"
	"github.com/getkin/kin-openapi/openapi3"
)

func main() {
	extractor, err := files.OpenApiFile.Extractor()
	goutils.MustBeNil(err)

	getSchemas, err := extractor.ExtractListSchemas(api.GET,
		api.List.WithArrayLocator(api.ArrayLocationAtData),
	)
	goutils.MustBeNil(err)

	// Only the collections this connector reads.
	readObjects := filters.NewRules(
		"/customers",
		"/subscriptions",
		"/invoices",
		"/plans",
		"/items",
		"/credit_notes",
	)

	pipe := pipeline.NewSchemaPipe(getSchemas).
		Filter(filters.KeepWithPath(filters.AndPathMatcher{
			filters.NoIdPath{}, filters.NewAllowPathStrategy(readObjects),
		})).
		Reduce(reducers.ShortestNameFromUrlWithFullFallback).
		Map(mapping.DisplayNameFromObjectName).
		Map(mapping.DisplayNameFormat(
			mapping.CamelCaseToSpaceSeparated,
			mapping.CapitalizeFirstLetterEveryWord,
		)).
		Map(mapping.Fields(typedField, displayNamedField))

	output.PrintUnusedRules("readObjects", readObjects)
	goutils.MustBeNil(output.WriteMetadata(files.OutputDir, pipe))
	goutils.MustBeNil(output.WriteQueryParamStats(files.OutputDir, pipe))
	goutils.MustBeNil(output.WriteEndpoints(files.OutputDir, &pipe, nil, nil, nil))
}

// typedField types a field by its schema. Orb marks nullable fields as `oneOf: [<schema>, {type: null}]`,
// which the library leaves untyped, so the non-null option is used. Timestamps are `format: date-time`.
func typedField(field spec.Field) spec.Field {
	property := propertySchema(field)
	if property == nil {
		return field
	}

	if option := nonNullOption(property); option != nil {
		property = option
	}

	if property.Type == nil || len(property.Type.Slice()) != 1 {
		return field
	}

	field.Type = property.Type.Slice()[0]
	field.ValueOptions = enumOptions(property)

	switch {
	case field.Type == openapi3.TypeString && property.Format == "date-time":
		field.ValueType = common.ValueTypeDateTime
	case field.Type == openapi3.TypeString && property.Format == "date":
		field.ValueType = common.ValueTypeDate
	case field.Type == openapi3.TypeNumber:
		field.ValueType = common.ValueTypeFloat
	}

	return field
}

// displayNamedField writes the snake_case field name in words: created_at -> Created At.
func displayNamedField(field spec.Field) spec.Field {
	field.DisplayName = mapping.CapitalizeFirstLetterEveryWord(strings.ReplaceAll(field.Name, "_", " "))

	return field
}

func propertySchema(field spec.Field) *openapi3.Schema {
	if field.Origin.Schema == nil {
		return nil
	}

	property, ok := field.Origin.Schema.Properties[field.Name]
	if !ok || property.Value == nil {
		return nil
	}

	return property.Value
}

// nonNullOption returns X from `oneOf: [X, {type: null}]`, or nil for any other shape.
func nonNullOption(schema *openapi3.Schema) *openapi3.Schema {
	if len(schema.OneOf) != 2 { // nolint:mnd
		return nil
	}

	var option *openapi3.Schema

	for _, ref := range schema.OneOf {
		if ref.Value == nil {
			return nil
		}

		if ref.Value.Type != nil && ref.Value.Type.Is(openapi3.TypeNull) {
			continue
		}

		option = ref.Value
	}

	return option
}

func enumOptions(schema *openapi3.Schema) []string {
	options := make([]string, 0, len(schema.Enum))

	for _, value := range schema.Enum {
		if text, ok := value.(string); ok {
			options = append(options, text)
		}
	}

	if len(options) == 0 {
		return nil
	}

	return options
}
