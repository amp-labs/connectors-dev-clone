package main

import (
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/naming"
	"github.com/amp-labs/connectors/internal/goutils"
	"github.com/amp-labs/connectors/scripts/openapi/internal/api"
	"github.com/amp-labs/connectors/scripts/openapi/internal/api/filters"
	mapping "github.com/amp-labs/connectors/scripts/openapi/internal/api/map"
	"github.com/amp-labs/connectors/scripts/openapi/internal/api/map/mappingfields"
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

	// Only the collections this connector reads. Paths are relative to the spec's server,
	// https://api.withorb.com/v1; the connector adds the version.
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
			naming.SeparateUnderscoreWords,
			mapping.CapitalizeFirstLetterEveryWord,
		)).
		Map(mapping.Fields(
			typedField,
			mappingfields.DisplayNameFormat(func(_, name string) string {
				return mapping.CapitalizeFirstLetterEveryWord(naming.SeparateUnderscoreWords(name))
			}),
		))

	output.PrintUnusedRules("readObjects", readObjects)
	goutils.MustBeNil(output.WriteMetadata(files.OutputDir, pipe))
	goutils.MustBeNil(output.WriteQueryParamStats(files.OutputDir, pipe))
	goutils.MustBeNil(output.WriteEndpoints(files.OutputDir, &pipe, nil, nil, nil))
}

// typedField types a field by its property schema. Orb marks a nullable field as
// `oneOf: [<schema>, {type: null}]`, which the extractor leaves untyped, so the non-null option
// stands for the field. Timestamps are strings with format date-time, and numbers are floats.
func typedField(field spec.Field) spec.Field {
	if field.Origin.Schema == nil {
		return field
	}

	property, ok := field.Origin.Schema.Properties[field.Name]
	if !ok || property.Value == nil {
		return field
	}

	schema := nonNullOption(property.Value)

	switch {
	case schema.Type == nil:
		// A $ref to an object, or a union of objects.
		field.Type = openapi3.TypeObject
	case schema.Type.Is(openapi3.TypeString):
		field.Type = openapi3.TypeString
		field.ValueOptions = enumValues(schema)
		field.ValueType = formatValueType(schema.Format)
	default:
		field.Type = schema.Type.Slice()[0]
	}

	if field.Type == openapi3.TypeNumber {
		field.ValueType = common.ValueTypeFloat
	}

	return field
}

func nonNullOption(schema *openapi3.Schema) *openapi3.Schema {
	options := make([]*openapi3.Schema, 0, len(schema.OneOf))

	for _, option := range schema.OneOf {
		if option.Value != nil && (option.Value.Type == nil || !option.Value.Type.Is(openapi3.TypeNull)) {
			options = append(options, option.Value)
		}
	}

	if len(options) == 1 {
		return options[0]
	}

	return schema
}

func enumValues(schema *openapi3.Schema) []string {
	values := make([]string, 0, len(schema.Enum))

	for _, value := range schema.Enum {
		if text, ok := value.(string); ok {
			values = append(values, text)
		}
	}

	return values
}

func formatValueType(format string) common.ValueType {
	switch format {
	case "date-time":
		return common.ValueTypeDateTime
	case "date":
		return common.ValueTypeDate
	default:
		return ""
	}
}
