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
	responseNotFound := testutils.DataFromFile(t, "error-not-found.json")

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
							"email": {
								DisplayName:  "Email",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
								ReadOnly:     new(true),
							},
							"_updated_at": {
								DisplayName:  "Updated At",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "string:date-time",
								ReadOnly:     new(true),
							},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:       "Person combines its core schema with its own properties",
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
							"_id": {
								DisplayName:  "Record ID",
								ValueType:    common.ValueTypeString,
								ProviderType: "string:uuid",
								ReadOnly:     new(true),
							},
							"name": {
								DisplayName:  "Name",
								ValueType:    common.ValueTypeOther,
								ProviderType: "name",
							},
							"job_title": {
								DisplayName:  "Job Title",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
							"_updated_at": {
								DisplayName:  "Updated at",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "string:date-time",
								ReadOnly:     new(true),
							},
							"persona": {
								DisplayName:  "Persona",
								ValueType:    common.ValueTypeSingleSelect,
								ProviderType: "string",
								Values: []common.FieldValue{
									{Value: "Buyer", DisplayValue: "Buyer"},
									{Value: "Champion", DisplayValue: "Champion"},
								},
							},
							"company_id": {
								DisplayName:  "Company",
								ValueType:    common.ValueTypeReference,
								ProviderType: "many-to-one",
								ReferenceTo:  []string{"company"},
							},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:       "Deal maps JSON Schema types",
			Input:      []string{"deal"},
			Server:     schemasServer().Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"deal": {
						DisplayName: "Deals",
						Fields: map[string]common.FieldMetadata{
							"amount": {
								DisplayName:  "Amount",
								ValueType:    common.ValueTypeFloat,
								ProviderType: "number",
							},
							"close_date": {
								DisplayName:  "Close date",
								ValueType:    common.ValueTypeDate,
								ProviderType: "string:date",
							},
							"is_won": {
								DisplayName:  "Won",
								ValueType:    common.ValueTypeBoolean,
								ProviderType: "boolean",
							},
							"tags": {
								DisplayName:  "Tags",
								ValueType:    common.ValueTypeMultiSelect,
								ProviderType: "array",
								Values: []common.FieldValue{
									{Value: "Inbound", DisplayValue: "Inbound"},
									{Value: "Outbound", DisplayValue: "Outbound"},
								},
							},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:       "Object without a schema is reported, others still succeed",
			Input:      []string{"deal", "task"},
			Server:     schemasServer().Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"deal": {
						DisplayName: "Deals",
						Fields: map[string]common.FieldMetadata{
							"name": {
								DisplayName:  "Name",
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
			Name:  "Schemas error is reported per object",
			Input: []string{"company", "user"},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusNotFound, responseNotFound),
			}.Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"user": {
						DisplayName: "Users",
						Fields:      map[string]common.FieldMetadata{},
					},
				},
				Errors: map[string]error{
					"company": mockutils.ExpectedSubsetErrors{
						common.ErrNotFound,
						testutils.StringError("Workspace not found"),
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

func TestPersonMetadataSkipsToManyRelationships(t *testing.T) {
	t.Parallel()

	server := mockserver.Fixed{
		Setup:  mockserver.ContentJSON(),
		Always: mockserver.Response(http.StatusOK, testutils.DataFromFile(t, "schemas.json")),
	}.Server()
	defer server.Close()

	connector, err := constructTestConnector(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	result, err := connector.ListObjectMetadata(t.Context(), []string{"person"})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := result.Result["person"].Fields["deals"]; ok {
		t.Fatal("many-to-many `deals` is not a record attribute and must not be listed")
	}
}

func constructTestConnector(serverURL string) (*Connector, error) {
	connector, err := NewConnector(common.ConnectorParams{
		AuthenticatedClient: mockutils.NewClient(),
		Workspace:           "acme",
	})
	if err != nil {
		return nil, err
	}

	connector.SetBaseURL(mockutils.ReplaceURLOrigin(connector.HTTPClient().Base, serverURL))

	return connector, nil
}
