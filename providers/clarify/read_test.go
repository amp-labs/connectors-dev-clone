package clarify

import (
	"net/http"
	"strings"
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

	responseRateLimited := testutils.DataFromFile(t, "error-rate-limited.json")
	responseUserFirstPage := testutils.DataFromFile(t, "user-first-page.json")
	responseUserSecondPage := testutils.DataFromFile(t, "user-second-page.json")
	responseUserEmpty := testutils.DataFromFile(t, "user-empty.json")
	responsePersonFirstPage := testutils.DataFromFile(t, "person-first-page.json")

	// Spaces in operator names must reach Clarify as %20, not `+`.
	spacesAsPercent20 := mockcond.CustomCondition(func(_ http.ResponseWriter, r *http.Request) bool {
		return !strings.Contains(r.URL.RawQuery, "+") && strings.Contains(r.URL.RawQuery, "%20")
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
			Input:        common.ReadParams{ObjectName: "message", Fields: connectors.Fields("id")},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrOperationNotSupportedForObject},
		},
		{
			Name:  "Error response is parsed",
			Input: common.ReadParams{ObjectName: "person", Fields: connectors.Fields("id")},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusTooManyRequests, responseRateLimited),
			}.Server(),
			ExpectedErrs: []error{
				common.ErrLimitExceeded,
				testutils.StringError("API rate limit exceeded"),
			},
		},
		{
			Name: "First page of users is flattened and the next link is kept on the request's host",
			Input: common.ReadParams{
				ObjectName: "user",
				Fields:     connectors.Fields("id", "name", "roles", "email", "_updated_at"),
				PageSize:   2,
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/workspaces/withampersand/objects/user/resources"),
					mockcond.QueryParam("page[limit]", "2"),
					mockcond.QueryParam("page[offset]", "0"),
					mockcond.QueryParamsMissing("filter[_updated_at][Is on or after]",
						"filter[_updated_at][Is on or before]"),
				},
				Then: mockserver.Response(http.StatusOK, responseUserFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Id: "root",
					Fields: map[string]any{
						"id": "root",
						"name": map[string]any{
							"full_name":  "Clarify",
							"last_name":  "",
							"first_name": "Clarify",
						},
						"roles":       map[string]any{"items": []any{"admin"}},
						"email":       nil,
						"_updated_at": "2026-09-02T12:35:49.475Z",
					},
					Raw: map[string]any{
						"id":   "root",
						"type": "user",
					},
				}, {
					Id: "system",
					Fields: map[string]any{
						"id":    "system",
						"roles": map[string]any{"items": []any{"super_admin"}},
					},
					Raw: map[string]any{"id": "system"},
				}},
				NextPage: testconn.URLTestServer +
					"/v1/workspaces/withampersand/objects/user/resources?page%5Boffset%5D=2&page%5Blimit%5D=2",
				Done: false,
			},
		},
		{
			Name: "Next page link is fetched verbatim",
			Input: common.ReadParams{
				ObjectName: "user",
				Fields:     connectors.Fields("email"),
				NextPage: testconn.URLTestServer +
					"/v1/workspaces/withampersand/objects/user/resources?page%5Boffset%5D=2&page%5Blimit%5D=2",
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/workspaces/withampersand/objects/user/resources"),
					mockcond.QueryParam("page[limit]", "2"),
					mockcond.QueryParam("page[offset]", "2"),
				},
				Then: mockserver.Response(http.StatusOK, responseUserSecondPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Fields: map[string]any{"email": "april@withampersand.com"},
					Raw:    map[string]any{"type": "user"},
				}, {
					Fields: map[string]any{"email": "andy@whispered.com"},
					Raw:    map[string]any{"type": "user"},
				}},
				NextPage: testconn.URLTestServer +
					"/v1/workspaces/withampersand/objects/user/resources?page%5Boffset%5D=4&page%5Blimit%5D=2",
				Done: false,
			},
		},
		{
			Name: "Page size defaults to the maximum and a null next link ends the read",
			Input: common.ReadParams{
				ObjectName: "user",
				Fields:     connectors.Fields("id"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/workspaces/withampersand/objects/user/resources"),
					mockcond.QueryParam("page[limit]", "500"),
				},
				Then: mockserver.Response(http.StatusOK, responseUserEmpty),
			}.Server(),
			Comparator: testconn.ComparatorPagination,
			Expected:   &common.ReadResult{Rows: 0, NextPage: "", Done: true},
		},
		{
			Name: "Since is sent inclusively and applied exclusively to _updated_at",
			Input: common.ReadParams{
				ObjectName: "person",
				Fields:     connectors.Fields("id", "job_title", "company_id"),
				Since:      time.Date(2025, 8, 27, 18, 36, 13, 720_000_000, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/workspaces/withampersand/objects/person/resources"),
					mockcond.QueryParam("filter[_updated_at][Is on or after]", "2025-08-27T18:36:13.720Z"),
					mockcond.QueryParamsMissing("filter[_updated_at][Is on or before]"),
					spacesAsPercent20,
				},
				Then: mockserver.Response(http.StatusOK, responsePersonFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				// The first record was updated exactly at Since, so it is dropped.
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id: "001c628b-c77d-4449-b19c-3dec9a4d5efe",
					Fields: map[string]any{
						"id":         "001c628b-c77d-4449-b19c-3dec9a4d5efe",
						"job_title":  "Project Manager Program Management Office",
						"company_id": "7e1b02a5-c398-49b7-929f-3f9a088dee99",
					},
					Raw: map[string]any{"type": "person", "relationships": map[string]any{}},
				}},
				NextPage: testconn.URLTestServer +
					"/v1/workspaces/withampersand/objects/person/resources?page%5Boffset%5D=2&page%5Blimit%5D=2",
				Done: false,
			},
		},
		{
			Name: "Until is sent and applied inclusively to _updated_at",
			Input: common.ReadParams{
				ObjectName: "person",
				Fields:     connectors.Fields("id"),
				Until:      time.Date(2025, 8, 27, 18, 36, 13, 720_000_000, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/workspaces/withampersand/objects/person/resources"),
					mockcond.QueryParam("filter[_updated_at][Is on or before]", "2025-08-27T18:36:13.720Z"),
					mockcond.QueryParamsMissing("filter[_updated_at][Is on or after]"),
					spacesAsPercent20,
				},
				Then: mockserver.Response(http.StatusOK, responsePersonFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id:     "0009db0a-a4ba-48ed-a696-49b789131970",
					Fields: map[string]any{"id": "0009db0a-a4ba-48ed-a696-49b789131970"},
					Raw:    map[string]any{"id": "0009db0a-a4ba-48ed-a696-49b789131970"},
				}},
				NextPage: testconn.URLTestServer +
					"/v1/workspaces/withampersand/objects/person/resources?page%5Boffset%5D=2&page%5Blimit%5D=2",
				Done: false,
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
