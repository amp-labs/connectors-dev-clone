package clarify

import (
	"net/http"
	"testing"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockcond"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockserver"
	"github.com/amp-labs/connectors/test/utils/testconn"
	"github.com/amp-labs/connectors/test/utils/testutils"
)

func TestListObjectMetadata(t *testing.T) { // nolint:funlen
	t.Parallel()

	responseSchemas := testutils.DataFromFile(t, "schemas.json")
	responseUnauthorized := testutils.DataFromFile(t, "error-unauthorized.json")

	schemasServer := func() *mockserver.Conditional {
		return &mockserver.Conditional{
			Setup: mockserver.ContentJSON(),
			If: mockcond.And{
				mockcond.MethodGET(),
				mockcond.Path("/v1/workspaces/acme/schemas"),
			},
			Then: mockserver.Response(http.StatusOK, responseSchemas),
		}
	}

	tests := []testconn.TestCaseListObjectMetadata{
		{
			Name:         "At least one object name must be queried",
			Input:        nil,
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrMissingObjects},
		},
		{
			Name:       "Unknown object is reported per object",
			Input:      []string{"c_project"},
			Server:     mockserver.Dummy(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{},
				Errors: map[string]error{
					"c_project": mockutils.ExpectedSubsetErrors{common.ErrObjectNotSupported},
				},
			},
		},
		{
			Name:       "Users are described without a request",
			Input:      []string{"user"},
			Server:     mockserver.Dummy(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"user": {
						DisplayName: "Users",
						Fields: map[string]common.FieldMetadata{
							"id": {
								DisplayName:  "ID",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
								ReadOnly:     new(true),
							},
							"email": {
								DisplayName:  "Email",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
							"roles": {
								DisplayName:  "Roles",
								ValueType:    common.ValueTypeOther,
								ProviderType: "object",
							},
							"_updated_at": {
								DisplayName:  "Updated At",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "date-time",
								ReadOnly:     new(true),
							},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:       "People are described by the workspace schema",
			Input:      []string{"person"},
			Server:     schemasServer().Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"person": {
						DisplayName: "People",
						Fields: map[string]common.FieldMetadata{
							"id": {
								DisplayName:  "ID",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
								ReadOnly:     new(true),
							},
							"job_title": {
								DisplayName:  "Job Title",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
							"lifecycle_stage": {
								DisplayName:  "Lifecycle Stage",
								ValueType:    common.ValueTypeSingleSelect,
								ProviderType: "string",
								Values: []common.FieldValue{
									{Value: "Lead", DisplayValue: "Lead"},
									{Value: "Customer", DisplayValue: "Customer"},
								},
							},
							"company_id": {
								DisplayName:  "Company",
								ValueType:    common.ValueTypeReference,
								ProviderType: "string",
								ReferenceTo:  []string{"company"},
							},
							"deals": {
								DisplayName: "Deals",
								ValueType:   common.ValueTypeOther,
								ReferenceTo: []string{"deal"},
							},
							"birthday": {
								DisplayName:  "Birthday",
								ValueType:    common.ValueTypeDate,
								ProviderType: "date",
							},
							"score": {
								DisplayName:  "Score",
								ValueType:    common.ValueTypeFloat,
								ProviderType: "number",
							},
							"do_not_contact": {
								DisplayName:  "Do Not Contact",
								ValueType:    common.ValueTypeBoolean,
								ProviderType: "boolean",
							},
							// The schema's own definition wins over the system field fallback.
							"_updated_at": {
								DisplayName:  "Last Updated",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "date-time",
								ReadOnly:     new(true),
							},
							// Absent from the schema, added for every record.
							"_created_by": {
								DisplayName:  "Created By",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
								ReadOnly:     new(true),
							},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:       "Object without an entity schema is reported per object",
			Input:      []string{"person", "task"},
			Server:     schemasServer().Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"person": {
						DisplayName: "People",
						Fields: map[string]common.FieldMetadata{
							"job_title": {
								DisplayName:  "Job Title",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
						},
					},
				},
				Errors: map[string]error{
					"task": mockutils.ExpectedSubsetErrors{common.ErrMissingExpectedValues},
				},
			},
		},
		{
			Name:  "Failed schemas request fails only the schema objects",
			Input: []string{"deal", "user"},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnauthorized, responseUnauthorized),
			}.Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"user": {
						DisplayName: "Users",
						Fields: map[string]common.FieldMetadata{
							"email": {
								DisplayName:  "Email",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
						},
					},
				},
				Errors: map[string]error{
					"deal": mockutils.ExpectedSubsetErrors{
						common.ErrAccessToken,
						testutils.StringError("Invalid API key"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			tt.Run(t, func() (testconn.TestableMetadataReader, error) {
				return constructTestConnector(tt.Server.URL)
			})
		})
	}
}
