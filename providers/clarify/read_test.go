package clarify

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockcond"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockserver"
	"github.com/amp-labs/connectors/test/utils/testconn"
	"github.com/amp-labs/connectors/test/utils/testutils"
)

func TestRead(t *testing.T) { //nolint:funlen,maintidx
	t.Parallel()

	responseUnauthorized := testutils.DataFromFile(t, "error-unauthorized.json")
	responseNotFound := testutils.DataFromFile(t, "error-not-found.json")
	responsePersonFirstPage := testutils.DataFromFile(t, "person-first-page.json")
	responsePersonLastPage := testutils.DataFromFile(t, "person-last-page.json")
	responseUsers := testutils.DataFromFile(t, "users.json")

	personNextPage := "https://api.clarify.ai/v1/workspaces/acme/objects/person/resources?page%5Blimit%5D=2&page%5Boffset%5D=2"

	// Operator names carry spaces, which must reach Clarify as %20.
	spacesAsPercent20 := mockcond.CustomCondition(func(_ http.ResponseWriter, r *http.Request) bool {
		return strings.Contains(r.URL.RawQuery, "%5BIs%20after%5D") &&
			strings.Contains(r.URL.RawQuery, "%5BIs%20on%20or%20before%5D")
	})

	tests := []testconn.TestCaseRead{
		{
			Name:         "Read object must be included",
			Input:        common.ReadParams{Fields: connectors.Fields("id")},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrMissingObjects},
		},
		{
			Name:         "At least one field is requested",
			Input:        common.ReadParams{ObjectName: "person"},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrMissingFields},
		},
		{
			Name:         "Unknown object is not supported",
			Input:        common.ReadParams{ObjectName: "c_project", Fields: connectors.Fields("id")},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrOperationNotSupportedForObject},
		},
		{
			Name:  "Unauthorized error is parsed",
			Input: common.ReadParams{ObjectName: "person", Fields: connectors.Fields("id")},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnauthorized, responseUnauthorized),
			}.Server(),
			ExpectedErrs: []error{
				common.ErrAccessToken,
				testutils.StringError("Invalid API key"),
			},
		},
		{
			Name:  "Not found error is parsed",
			Input: common.ReadParams{ObjectName: "deal", Fields: connectors.Fields("id")},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusNotFound, responseNotFound),
			}.Server(),
			ExpectedErrs: []error{
				common.ErrNotFound,
				testutils.StringError("Workspace not found"),
			},
		},
		{
			Name: "First page of people is flattened and links to the next page",
			Input: common.ReadParams{
				ObjectName: "person",
				Fields:     connectors.Fields("id", "job_title", "name", "email_addresses", "_updated_at"),
				PageSize:   2,
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/workspaces/acme/objects/person/resources"),
					mockcond.QueryParam("page[limit]", "2"),
					mockcond.QueryParamsMissing("filter[_updated_at][Is after]", "filter[_updated_at][Is on or before]"),
				},
				Then: mockserver.Response(http.StatusOK, responsePersonFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Id: "5f8b7d2e-9c4a-4e1b-8f3d-2a6c9e0b4d71",
					Fields: map[string]any{
						"id":        "5f8b7d2e-9c4a-4e1b-8f3d-2a6c9e0b4d71",
						"job_title": "VP of Marketing",
						"name": map[string]any{
							"first_name": "Jane",
							"last_name":  "Doe",
						},
						"email_addresses": map[string]any{
							"items": []any{"jane@acme.com"},
						},
						"_updated_at": "2026-02-01T17:45:00.000Z",
					},
					Raw: map[string]any{
						"type": "person",
					},
				}, {
					Id:  "8a1c3e5f-2b4d-4f6a-9c8e-1d3f5a7b9c02",
					Raw: map[string]any{"id": "8a1c3e5f-2b4d-4f6a-9c8e-1d3f5a7b9c02", "type": "person"},
					Fields: map[string]any{
						"id":        "8a1c3e5f-2b4d-4f6a-9c8e-1d3f5a7b9c02",
						"job_title": nil,
					},
				}},
				NextPage: common.NextPageToken(personNextPage),
				Done:     false,
			},
		},
		{
			Name: "Next page is fetched verbatim and the last page ends the read",
			Input: common.ReadParams{
				ObjectName: "person",
				Fields:     connectors.Fields("job_title"),
				NextPage: testconn.URLTestServer +
					"/v1/workspaces/acme/objects/person/resources?page%5Blimit%5D=2&page%5Boffset%5D=2",
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/workspaces/acme/objects/person/resources"),
					mockcond.QueryParam("page[limit]", "2"),
					mockcond.QueryParam("page[offset]", "2"),
				},
				Then: mockserver.Response(http.StatusOK, responsePersonLastPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id:     "b7d9f1a3-4c6e-4a8b-8d0f-2e4a6c8e0b13",
					Raw:    map[string]any{"id": "b7d9f1a3-4c6e-4a8b-8d0f-2e4a6c8e0b13", "type": "person"},
					Fields: map[string]any{"job_title": "Engineer"},
				}},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Incremental read of people filters _updated_at on the server",
			Input: common.ReadParams{
				ObjectName: "person",
				Fields:     connectors.Fields("job_title"),
				Since:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Until:      time.Date(2026, 3, 1, 12, 30, 0, 500_000_000, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/workspaces/acme/objects/person/resources"),
					mockcond.QueryParam("page[limit]", "500"),
					mockcond.QueryParam("filter[_updated_at][Is after]", "2026-01-01T00:00:00.000Z"),
					mockcond.QueryParam("filter[_updated_at][Is on or before]", "2026-03-01T12:30:00.500Z"),
					spacesAsPercent20,
				},
				Then: mockserver.Response(http.StatusOK, responsePersonLastPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id:     "b7d9f1a3-4c6e-4a8b-8d0f-2e4a6c8e0b13",
					Raw:    map[string]any{"id": "b7d9f1a3-4c6e-4a8b-8d0f-2e4a6c8e0b13", "type": "person"},
					Fields: map[string]any{"job_title": "Engineer"},
				}},
				Done: true,
			},
		},
		{
			Name: "Users are read from the users endpoint",
			Input: common.ReadParams{
				ObjectName: "user",
				Fields:     connectors.Fields("email", "roles"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/workspaces/acme/users"),
					mockcond.QueryParam("page[limit]", "500"),
				},
				Then: mockserver.Response(http.StatusOK, responseUsers),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 3,
				Data: []common.ReadResultRow{{
					Id:  "7d4e2f9a-1b3c-4d5e-8f6a-9c0b2d4e6f81",
					Raw: map[string]any{"id": "7d4e2f9a-1b3c-4d5e-8f6a-9c0b2d4e6f81", "type": "user"},
					Fields: map[string]any{
						"email": "alex@acme.com",
						"roles": map[string]any{"items": []any{"member"}},
					},
				}, {
					Id:     "2c4e6a8b-0d1f-4b3d-9e5f-7a9c1e3b5d24",
					Raw:    map[string]any{"id": "2c4e6a8b-0d1f-4b3d-9e5f-7a9c1e3b5d24", "type": "user"},
					Fields: map[string]any{"email": "sam@acme.com"},
				}, {
					Id:     "9e1a3c5e-7b2d-4f4a-8c6e-0b2d4f6a8c35",
					Raw:    map[string]any{"id": "9e1a3c5e-7b2d-4f4a-8c6e-0b2d4f6a8c35", "type": "user"},
					Fields: map[string]any{"email": "kim@acme.com"},
				}},
				Done: true,
			},
		},
		{
			Name: "Incremental read of users filters _updated_at in the connector",
			Input: common.ReadParams{
				ObjectName: "user",
				Fields:     connectors.Fields("email"),
				// Since is exclusive: Sam, updated exactly at Since, is left out.
				Since: time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC),
				// Until is inclusive: Alex, updated exactly at Until, is kept.
				Until: time.Date(2026, 2, 1, 17, 45, 0, 0, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/workspaces/acme/users"),
					mockcond.QueryParamsMissing("filter[_updated_at][Is after]", "filter[_updated_at][Is on or before]"),
				},
				Then: mockserver.Response(http.StatusOK, responseUsers),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id:     "7d4e2f9a-1b3c-4d5e-8f6a-9c0b2d4e6f81",
					Raw:    map[string]any{"id": "7d4e2f9a-1b3c-4d5e-8f6a-9c0b2d4e6f81", "type": "user"},
					Fields: map[string]any{"email": "alex@acme.com"},
				}},
				Done: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			tt.Run(t, func() (testconn.TestableReader, error) {
				return constructTestConnector(tt.Server.URL)
			})
		})
	}
}

func TestNewConnectorRequiresWorkspace(t *testing.T) {
	t.Parallel()

	_, err := NewConnector(common.ConnectorParams{
		Module:              common.ModuleRoot,
		AuthenticatedClient: mockutils.NewClient(),
	})
	if err == nil {
		t.Fatal("expected an error for a missing workspace")
	}
}

func constructTestConnector(serverURL string) (*Connector, error) {
	connector, err := NewConnector(common.ConnectorParams{
		Module:              common.ModuleRoot,
		AuthenticatedClient: mockutils.NewClient(),
		Workspace:           "acme",
	})
	if err != nil {
		return nil, err
	}

	connector.SetBaseURL(mockutils.ReplaceURLOrigin(connector.HTTPClient().Base, serverURL))

	return connector, nil
}
