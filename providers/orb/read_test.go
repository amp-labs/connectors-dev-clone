package orb

import (
	"net/http"
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
	responseCustomersFirstPage := testutils.DataFromFile(t, "customers-first-page.json")
	responseCustomersLastPage := testutils.DataFromFile(t, "customers-last-page.json")
	responseCoupons := testutils.DataFromFile(t, "coupons.json")
	responseEmpty := testutils.DataFromFile(t, "empty-page.json")

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
			Input:        common.ReadParams{ObjectName: "events", Fields: connectors.Fields("id")},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrOperationNotSupportedForObject},
		},
		{
			Name:  "Error response is returned with its detail",
			Input: common.ReadParams{ObjectName: "customers", Fields: connectors.Fields("id")},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnauthorized, responseUnauthorized),
			}.Server(),
			ExpectedErrs: []error{
				common.ErrAccessToken,
				testutils.StringError("Your request did not provide a valid or well-formatted API key."),
			},
		},
		{
			Name: "First page of customers links to the next cursor",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name", "balance", "billing_address"),
				PageSize:   2,
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/customers"),
					mockcond.QueryParam("limit", "2"),
					mockcond.QueryParamsMissing("cursor"),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Id: "XuxCbt7x9L82yyeF",
					Fields: map[string]any{
						"name":    "Acme Corp",
						"balance": "0.00",
						"billing_address": map[string]any{
							"line1":       "1 Market St",
							"line2":       nil,
							"city":        "San Francisco",
							"state":       "CA",
							"postal_code": "94105",
							"country":     "US",
						},
					},
					Raw: map[string]any{
						"external_customer_id": "acme-001",
					},
				}, {
					Id: "Ab3kLmN9pQrSt7uV",
					Fields: map[string]any{
						"name":            "Globex",
						"balance":         "-12.50",
						"billing_address": nil,
					},
					Raw: map[string]any{
						"payment_provider": "stripe_charge",
					},
				}},
				NextPage: testconn.URLTestServer + "/v1/customers?cursor=Y3Vyc29yLTI%3D&limit=2",
				Done:     false,
			},
		},
		{
			Name: "Page without more results is the last one",
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
					Id:     "Zz9yXw8vUt7sRq6P",
					Fields: map[string]any{"name": "Initech"},
					Raw:    map[string]any{"currency": "EUR"},
				}},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Page size defaults to the maximum of 100",
			Input: common.ReadParams{
				ObjectName: "credit_notes",
				Fields:     connectors.Fields("id"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/credit_notes"),
					mockcond.QueryParam("limit", "100"),
				},
				Then: mockserver.Response(http.StatusOK, responseEmpty),
			}.Server(),
			Expected: &common.ReadResult{Rows: 0, Data: []common.ReadResultRow{}, Done: true},
		},
		{
			Name: "Coupons include archived ones",
			Input: common.ReadParams{
				ObjectName: "coupons",
				Fields:     connectors.Fields("redemption_code", "archived_at"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/coupons"),
					mockcond.QueryParam("show_archived", "true"),
				},
				Then: mockserver.Response(http.StatusOK, responseCoupons),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id: "7iz2yanVjQoBZhyH",
					Fields: map[string]any{
						"redemption_code": "HALFOFF",
						"archived_at":     "2026-09-30T00:00:00+00:00",
					},
					Raw: map[string]any{"times_redeemed": float64(3)},
				}},
				Done: true,
			},
		},
		{
			Name: "Since and Until are not sent because no list endpoint filters on an update time",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name"),
				Since:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
				Until:      time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/customers"),
					mockcond.QueryParamsMissing(
						"created_at[gt]", "created_at[gte]", "created_at[lt]", "created_at[lte]",
					),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersLastPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id:     "Zz9yXw8vUt7sRq6P",
					Fields: map[string]any{"name": "Initech"},
					Raw:    map[string]any{"created_at": "2026-08-15T12:00:00+00:00"},
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

func constructTestConnector(serverURL string) (*Connector, error) {
	connector, err := NewConnector(common.ConnectorParams{
		Module:              common.ModuleRoot,
		AuthenticatedClient: mockutils.NewClient(),
	})
	if err != nil {
		return nil, err
	}

	connector.SetBaseURL(mockutils.ReplaceURLOrigin(connector.HTTPClient().Base, serverURL))

	return connector, nil
}
