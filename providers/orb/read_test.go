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

func TestRead(t *testing.T) { //nolint:funlen,maintidx
	t.Parallel()

	responseUnauthorized := testutils.DataFromFile(t, "error-unauthorized.json")
	responseValidation := testutils.DataFromFile(t, "error-validation.json")
	responseCustomersFirstPage := testutils.DataFromFile(t, "customers-first-page.json")
	responseCustomersLastPage := testutils.DataFromFile(t, "customers-last-page.json")
	responseInvoices := testutils.DataFromFile(t, "invoices.json")

	since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 9, 30, 12, 30, 15, 250_000_000, time.UTC)

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
			Input:        common.ReadParams{ObjectName: "prices", Fields: connectors.Fields("id")},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrOperationNotSupportedForObject},
		},
		{
			Name:  "Authentication error is parsed",
			Input: common.ReadParams{ObjectName: "customers", Fields: connectors.Fields("id")},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnauthorized, responseUnauthorized),
			}.Server(),
			ExpectedErrs: []error{
				common.ErrAccessToken,
				testutils.StringError("Authentication failure: Your request did not provide a valid or well-formatted API key."),
			},
		},
		{
			Name:  "Validation error is parsed",
			Input: common.ReadParams{ObjectName: "plans", Fields: connectors.Fields("id")},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusBadRequest, responseValidation),
			}.Server(),
			ExpectedErrs: []error{
				common.ErrBadRequest,
				testutils.StringError("Request validation error: Invalid query parameters."),
			},
		},
		{
			Name: "First page of customers links to the next cursor",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name", "email", "balance"),
				PageSize:   2,
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/customers"),
					mockcond.QueryParam("limit", "2"),
					mockcond.QueryParamsMissing("cursor", "created_at[gt]", "created_at[lte]"),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Id: "XimGiw3pnsgusvc3",
					Fields: map[string]any{
						"name":    "Acme Corp",
						"email":   "billing@acme.example",
						"balance": "0.00",
					},
					Raw: map[string]any{
						"external_customer_id": "acme-001",
					},
				}, {
					Id: "3QMnasWSbFE87Hvd",
					Fields: map[string]any{
						"name":    "Globex",
						"email":   "ap@globex.example",
						"balance": "12.50",
					},
					Raw: map[string]any{
						"metadata": map[string]any{"tier": "gold"},
					},
				}},
				NextPage: testconn.URLTestServer + "/v1/customers?cursor=Y3Vyc29yLTI%3D&limit=2",
				Done:     false,
			},
		},
		{
			Name: "Last page of customers has no next page",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name"),
				NextPage:   testconn.URLTestServer + "/v1/customers?cursor=Y3Vyc29yLTI%3D&limit=2",
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/customers"),
					mockcond.QueryParam("limit", "2"),
					mockcond.QueryParam("cursor", "Y3Vyc29yLTI="),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersLastPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id:     "Kq8Vt2ZbLm4Rx9Nc",
					Fields: map[string]any{"name": "Initech"},
					Raw:    map[string]any{"id": "Kq8Vt2ZbLm4Rx9Nc"},
				}},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Incremental read of customers sends created_at bounds and keeps them on the next page",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name"),
				Since:      since,
				Until:      until,
				PageSize:   500,
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/customers"),
					mockcond.QueryParam("limit", "100"),
					mockcond.QueryParam("created_at[gt]", "2026-09-01T00:00:00.000Z"),
					mockcond.QueryParam("created_at[lte]", "2026-09-30T12:30:15.250Z"),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorPagination,
			Expected: &common.ReadResult{
				Rows: 2,
				NextPage: testconn.URLTestServer + "/v1/customers?" +
					"created_at%5Bgt%5D=2026-09-01T00%3A00%3A00.000Z&" +
					"created_at%5Blte%5D=2026-09-30T12%3A30%3A15.250Z&" +
					"cursor=Y3Vyc29yLTI%3D&limit=100",
				Done: false,
			},
		},
		{
			Name: "Invoices have no created_at filter and are read in full",
			Input: common.ReadParams{
				ObjectName: "invoices",
				Fields:     connectors.Fields("status", "total", "customer"),
				Since:      since,
				Until:      until,
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/invoices"),
					mockcond.QueryParam("limit", "100"),
					mockcond.QueryParamsMissing("created_at[gt]", "created_at[lte]"),
				},
				Then: mockserver.Response(http.StatusOK, responseInvoices),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id: "rUHdhmg45vY45DX",
					Fields: map[string]any{
						"status": "issued",
						"total":  "8.00",
						"customer": map[string]any{
							"id":                   "3QMnasWSbFE87Hvd",
							"external_customer_id": nil,
						},
					},
					Raw: map[string]any{"invoice_number": "JYEFHK-00001"},
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
