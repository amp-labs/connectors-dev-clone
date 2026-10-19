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
	responseRateLimited := testutils.DataFromFile(t, "error-rate-limited.json")

	tests := []testconn.TestCaseListObjectMetadata{
		{
			Name:         "At least one object name must be queried",
			Input:        nil,
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrMissingObjects},
		},
		{
			Name:       "Unknown object is reported per object",
			Input:      []string{"message"},
			Server:     mockserver.Dummy(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{},
				Errors: map[string]error{
					"message": mockutils.ExpectedSubsetErrors{common.ErrObjectNotSupported},
				},
			},
		},
		{
			Name:  "Objects are described by their workspace schemas",
			Input: []string{"person", "user", "message"},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/workspaces/withampersand/schemas"),
				},
				Then: mockserver.Response(http.StatusOK, responseSchemas),
			}.Server(),
			// To-many relationships are not returned as attributes, so they are not fields.
			Comparator: testconn.ComparatorSubsetMetadataWithMissingFields(map[string][]string{
				"person": {"deals", "tasks", "meetings", "messages", "attachments", "form_submissions"},
				"user":   {"meetings", "owned_deals", "created_people"},
			}),
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
								DisplayName:  "Job title",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
							"name": {
								DisplayName:  "Name",
								ValueType:    common.ValueTypeOther,
								ProviderType: "personName",
							},
							"email_addresses": {
								DisplayName:  "Email addresses",
								ValueType:    common.ValueTypeOther,
								ProviderType: "collectionOfEmails",
							},
							"company_id": {
								DisplayName:  "Company",
								ValueType:    common.ValueTypeReference,
								ProviderType: "uuid",
								ReferenceTo:  []string{"company"},
							},
							"_created_by": {
								DisplayName:  "Created by",
								ValueType:    common.ValueTypeReference,
								ProviderType: "string",
								ReadOnly:     new(true),
								ReferenceTo:  []string{"user"},
							},
							"_updated_at": {
								DisplayName:  "Updated at",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "date-time",
								ReadOnly:     new(true),
							},
						},
					},
					"user": {
						DisplayName: "Users",
						Fields: map[string]common.FieldMetadata{
							"email": {
								DisplayName:  "Email",
								ValueType:    common.ValueTypeString,
								ProviderType: "email",
								ReadOnly:     new(true),
							},
							"roles": {
								DisplayName:  "Roles",
								ValueType:    common.ValueTypeMultiSelect,
								ProviderType: "object",
								ReadOnly:     nil,
								Values: []common.FieldValue{
									{Value: "super_admin", DisplayValue: "super_admin"},
									{Value: "admin", DisplayValue: "admin"},
									{Value: "member", DisplayValue: "member"},
								},
							},
						},
					},
				},
				Errors: map[string]error{
					"message": mockutils.ExpectedSubsetErrors{common.ErrObjectNotSupported},
				},
			},
		},
		{
			Name:  "Schemas error fails the call",
			Input: []string{"deal"},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusTooManyRequests, responseRateLimited),
			}.Server(),
			ExpectedErrs: []error{
				common.ErrLimitExceeded,
				testutils.StringError("API rate limit exceeded"),
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

func constructTestConnector(serverURL string) (*Connector, error) {
	connector, err := NewConnector(common.ConnectorParams{
		Module:              common.ModuleRoot,
		AuthenticatedClient: mockutils.NewClient(),
		Workspace:           "withampersand",
	})
	if err != nil {
		return nil, err
	}

	connector.SetBaseURL(mockutils.ReplaceURLOrigin(connector.HTTPClient().Base, serverURL))

	return connector, nil
}
