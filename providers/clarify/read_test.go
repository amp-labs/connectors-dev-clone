package clarify

import (
	"net/http"
	"testing"
	"time"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockcond"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockserver"
	"github.com/amp-labs/connectors/test/utils/testconn"
	"github.com/amp-labs/connectors/test/utils/testutils"
)

func TestRead(t *testing.T) { //nolint:funlen,maintidx
	t.Parallel()

	responseNotFound := testutils.DataFromFile(t, "error-not-found.json")
	responseValidation := testutils.DataFromFile(t, "error-validation.json")
	responsePersonFirstPage := testutils.DataFromFile(t, "person-first-page.json")
	responsePersonLastPage := testutils.DataFromFile(t, "person-last-page.json")
	responseUsers := testutils.DataFromFile(t, "users.json")

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
			Name:  "Not found error is parsed",
			Input: common.ReadParams{ObjectName: "person", Fields: connectors.Fields("id")},
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
			Name:  "Validation error is parsed",
			Input: common.ReadParams{ObjectName: "deal", Fields: connectors.Fields("id")},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnprocessableEntity, responseValidation),
			}.Server(),
			ExpectedErrs: []error{
				testutils.StringError("Invalid input: /page/limit: must be <= 500"),
			},
		},
		{
			Name: "First page of people is flattened, filtered by _updated_at and links to the next page",
			Input: common.ReadParams{
				ObjectName: "person",
				Fields:     connectors.Fields("id", "job_title", "company_id", "name", "type", "_updated_at"),
				PageSize:   2,
				Since:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				Until:      time.Date(2026, 3, 1, 12, 30, 0, 500_000_000, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/workspaces/acme/objects/person/resources"),
					mockcond.QueryParam("page[limit]", "2"),
					mockcond.QueryParam("page[offset]", "0"),
					mockcond.QueryParam("filter[_updated_at][Is after]", "2025-01-01T00:00:00.000Z"),
					mockcond.QueryParam("filter[_updated_at][Is on or before]", "2026-03-01T12:30:00.500Z"),
				},
				Then: mockserver.Response(http.StatusOK, responsePersonFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Id: "0009db0a-a4ba-48ed-a696-49b789131970",
					Fields: map[string]any{
						"id":          "0009db0a-a4ba-48ed-a696-49b789131970",
						"job_title":   "VP of Marketing",
						"company_id":  "0e69c577-f8c4-45c6-8570-9a122e0fd08a",
						"type":        "person",
						"_updated_at": "2025-08-27T18:36:13.720Z",
						"name": map[string]any{
							"full_name":  "Jane Doe",
							"first_name": "Jane",
							"last_name":  "Doe",
						},
					},
					// Raw keeps the JSON:API shape.
					Raw: map[string]any{
						"id":            "0009db0a-a4ba-48ed-a696-49b789131970",
						"type":          "person",
						"relationships": map[string]any{},
					},
				}, {
					Id: "5f8b7d2e-9c4a-4e1b-8f3d-2a6c9e0b4d71",
					Fields: map[string]any{
						"id":         "5f8b7d2e-9c4a-4e1b-8f3d-2a6c9e0b4d71",
						"job_title":  nil,
						"company_id": nil,
						// An attribute named like a resource member wins over it.
						"type": "Partner",
					},
					Raw: map[string]any{"type": "person"},
				}},
				// links.next comes back as http://api.clarify.ai/...; only its path and query are kept.
				NextPage: testconn.URLTestServer + "/v1/workspaces/acme/objects/person/resources" +
					"?page%5Boffset%5D=2&page%5Blimit%5D=2" +
					"&filter%5B_updated_at%5D%5BIs%20after%5D=2025-01-01T00%3A00%3A00.000Z",
				Done: false,
			},
		},
		{
			Name: "Next page link is followed as given until links.next is null",
			Input: common.ReadParams{
				ObjectName: "person",
				Fields:     connectors.Fields("id", "job_title"),
				NextPage: testconn.URLTestServer + "/v1/workspaces/acme/objects/person/resources" +
					"?page%5Boffset%5D=2&page%5Blimit%5D=2",
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
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
					Id: "7a1c2e3d-4b5f-4a6b-8c7d-9e0f1a2b3c4d",
					Fields: map[string]any{
						"id":        "7a1c2e3d-4b5f-4a6b-8c7d-9e0f1a2b3c4d",
						"job_title": "CTO",
					},
					Raw: map[string]any{"id": "7a1c2e3d-4b5f-4a6b-8c7d-9e0f1a2b3c4d"},
				}},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Users are read in full with the maximum page size",
			Input: common.ReadParams{
				ObjectName: "user",
				Fields:     connectors.Fields("email", "roles", "is_service_account"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/workspaces/acme/users"),
					mockcond.QueryParam("page[limit]", "500"),
					mockcond.QueryParam("page[offset]", "0"),
				},
				Then: mockserver.Response(http.StatusOK, responseUsers),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Id: "user_01JZKRMYJMFWPFW74MD3G31998",
					Fields: map[string]any{
						"email":              "alex@acme.com",
						"roles":              map[string]any{"items": []any{"admin"}},
						"is_service_account": false,
					},
					Raw: map[string]any{"type": "user"},
				}, {
					Id: "user_01K1VN53JRP761BANN7KDQ8AVK",
					Fields: map[string]any{
						"email": "sam@acme.com",
					},
					Raw: map[string]any{"id": "user_01K1VN53JRP761BANN7KDQ8AVK"},
				}},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Users are filtered by _updated_at in the connector, Since exclusive",
			Input: common.ReadParams{
				ObjectName: "user",
				Fields:     connectors.Fields("email"),
				Since:      time.Date(2025, 9, 10, 19, 53, 51, 989_000_000, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/workspaces/acme/users"),
					mockcond.QueryParamsMissing("filter[_updated_at][Is after]"),
				},
				Then: mockserver.Response(http.StatusOK, responseUsers),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id: "user_01JZKRMYJMFWPFW74MD3G31998",
					Fields: map[string]any{
						"email": "alex@acme.com",
					},
					Raw: map[string]any{"id": "user_01JZKRMYJMFWPFW74MD3G31998"},
				}},
				NextPage: "",
				Done:     true,
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
