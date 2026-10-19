package orb

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

func TestRead(t *testing.T) { //nolint:funlen
	t.Parallel()

	responseCustomersFirstPage := testutils.DataFromFile(t, "customers-first-page.json")
	responseCustomersSecondPage := testutils.DataFromFile(t, "customers-second-page.json")
	responseItemsLastPage := testutils.DataFromFile(t, "items-last-page.json")
	responseSubscriptions := testutils.DataFromFile(t, "subscriptions-first-page.json")

	tests := []testconn.TestCaseRead{
		{
			Name:         "Read object must be included",
			Input:        common.ReadParams{Fields: connectors.Fields("id")},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrMissingObjects},
		},
		{
			Name:         "At least one field is requested",
			Input:        common.ReadParams{ObjectName: "customers"},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrMissingFields},
		},
		{
			Name:         "Unknown object is not supported",
			Input:        common.ReadParams{ObjectName: "coupons", Fields: connectors.Fields("id")},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrOperationNotSupportedForObject},
		},
		{
			Name:  "Invalid API key is an access token error",
			Input: common.ReadParams{ObjectName: "customers", Fields: connectors.Fields("id")},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnauthorized),
			}.Server(),
			ExpectedErrs: []error{common.ErrAccessToken},
		},
		{
			Name: "First page of customers links to the next cursor",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name", "external_customer_id", "balance"),
				PageSize:   2,
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/customers"),
					mockcond.QueryParam("limit", "2"),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Id: "eN3gvMFjPt5FjLbc",
					Fields: map[string]any{
						"name":                 "11x",
						"external_customer_id": nil,
						"balance":              "0.00",
					},
					Raw: map[string]any{"email": "11x@11x.ai"},
				}, {
					Id: "NLGwoEaecJENHwSF",
					Fields: map[string]any{
						"name":                 "staging-probe",
						"external_customer_id": "53939f75-7988-4855-beab-22b5ecceba43",
						"balance":              "0.00",
					},
					Raw: map[string]any{"timezone": "Etc/UTC"},
				}},
				NextPage: testconn.URLTestServer + "/v1/customers?cursor=cwYu8K4RUVLjNJLn&limit=2",
				Done:     false,
			},
		},
		{
			Name: "Next page replaces the cursor and keeps the limit",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name"),
				NextPage:   testconn.URLTestServer + "/v1/customers?cursor=cwYu8K4RUVLjNJLn&limit=2",
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/customers"),
					mockcond.QueryParam("limit", "2"),
					mockcond.QueryParam("cursor", "cwYu8K4RUVLjNJLn"),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersSecondPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Id:     "cwYu8K4RUVLjNJLn",
					Fields: map[string]any{"name": "dev-probe"},
					Raw:    map[string]any{"id": "cwYu8K4RUVLjNJLn"},
				}, {
					Id:     "Ta6iGU8sBznEAPPV",
					Fields: map[string]any{"name": "Lauren Inc."},
					Raw:    map[string]any{"id": "Ta6iGU8sBznEAPPV"},
				}},
				NextPage: testconn.URLTestServer + "/v1/customers?cursor=3QMnasWSbFE87Hvd&limit=2",
				Done:     false,
			},
		},
		{
			Name: "Page without more records ends the read and default page size is the maximum",
			Input: common.ReadParams{
				ObjectName: "items",
				Fields:     connectors.Fields("name", "archived_at"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/items"),
					mockcond.QueryParam("limit", "100"),
				},
				Then: mockserver.Response(http.StatusOK, responseItemsLastPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 38,
				Data: []common.ReadResultRow{{
					Id: "QAhcSngCq7Z4YdAw",
					Fields: map[string]any{
						"name":        "claude-grouppool-cr-20261014221700-item",
						"archived_at": "2026-10-14T22:21:02+00:00",
					},
					Raw: map[string]any{"created_at": "2026-10-14T22:17:01+00:00"},
				}},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Since and Until are not sent and the page size is capped at 100",
			Input: common.ReadParams{
				ObjectName: "subscriptions",
				Fields:     connectors.Fields("status", "billing_cycle_day"),
				PageSize:   500,
				Since:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
				Until:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/subscriptions"),
					mockcond.QueryParam("limit", "100"),
					mockcond.QueryParamsMissing("created_at[gt]", "created_at[lte]", "cursor"),
				},
				Then: mockserver.Response(http.StatusOK, responseSubscriptions),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Id: "DQ8hBPBugWDjFCFk",
					Fields: map[string]any{
						"status":            "active",
						"billing_cycle_day": float64(1),
					},
					Raw: map[string]any{"name": "Omilia"},
				}},
				NextPage: testconn.URLTestServer + "/v1/subscriptions?cursor=QbRDHD2Nogt74Wgj&limit=100",
				Done:     false,
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
