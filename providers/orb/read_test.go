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

	// The docs' example error body; the live tests have not returned an error yet.
	// https://docs.withorb.com/api-reference/error-responses
	responseNotFound := testutils.DataFromFile(t, "error-not-found.json")
	responseCustomersFirstPage := testutils.DataFromFile(t, "customers-first-page.json")
	responseCustomersEmpty := testutils.DataFromFile(t, "customers-empty.json")
	responseItemsFirstPage := testutils.DataFromFile(t, "items-first-page.json")
	responseItemsAll := testutils.DataFromFile(t, "items-all.json")

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
			Name:  "Error response is parsed",
			Input: common.ReadParams{ObjectName: "plans", Fields: connectors.Fields("id")},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusNotFound, responseNotFound),
			}.Server(),
			ExpectedErrs: []error{
				common.ErrNotFound,
				testutils.StringError("A specified resource was not found.: " +
					"Could not find a resource of type Plan with identifier: DBhwNcMNfYgeBpn."),
			},
		},
		{
			Name: "First page of customers returns records and the next cursor",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name", "email", "external_customer_id"),
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
					Id: "eN3gvMFjPt5FjLbc",
					Fields: map[string]any{
						"name":                 "11x",
						"email":                "11x@11x.ai",
						"external_customer_id": nil,
					},
					Raw: map[string]any{
						"created_at": "2026-10-16T13:49:57+00:00",
					},
				}, {
					Id: "NLGwoEaecJENHwSF",
					Fields: map[string]any{
						"name":                 "staging-probe",
						"external_customer_id": "53939f75-7988-4855-beab-22b5ecceba43",
					},
					Raw: map[string]any{
						"balance": "0.00",
					},
				}},
				NextPage: "cwYu8K4RUVLjNJLn",
				Done:     false,
			},
		},
		{
			Name: "Next page sends the cursor and the page size is capped at 100",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name"),
				PageSize:   500,
				NextPage:   "cwYu8K4RUVLjNJLn",
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/customers"),
					mockcond.QueryParam("limit", "100"),
					mockcond.QueryParam("cursor", "cwYu8K4RUVLjNJLn"),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersEmpty),
			}.Server(),
			Expected: &common.ReadResult{
				Rows:     0,
				Data:     []common.ReadResultRow{},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Incremental read of customers filters on created_at",
			Input: common.ReadParams{
				ObjectName: "customers",
				Fields:     connectors.Fields("name"),
				Since:      time.Date(2026, 10, 19, 11, 11, 24, 186000000, time.UTC),
				Until:      time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/customers"),
					mockcond.QueryParam("limit", "100"),
					mockcond.QueryParam("created_at[gt]", "2026-10-19T11:11:24.186Z"),
					mockcond.QueryParam("created_at[lte]", "2026-10-20T00:00:00.000Z"),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersEmpty),
			}.Server(),
			Expected: &common.ReadResult{
				Rows:     0,
				Data:     []common.ReadResultRow{},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Incremental read of invoices filters on invoice_date",
			Input: common.ReadParams{
				ObjectName: "invoices",
				Fields:     connectors.Fields("id"),
				Since:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
				Until:      time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/invoices"),
					mockcond.QueryParam("invoice_date[gt]", "2026-10-01T00:00:00.000Z"),
					mockcond.QueryParam("invoice_date[lte]", "2026-10-02T00:00:00.000Z"),
					mockcond.QueryParamsMissing("created_at[gt]", "created_at[lte]"),
				},
				Then: mockserver.Response(http.StatusOK, responseCustomersEmpty),
			}.Server(),
			Expected: &common.ReadResult{
				Rows:     0,
				Data:     []common.ReadResultRow{},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Items are filtered on created_at by the connector and Since is exclusive",
			Input: common.ReadParams{
				ObjectName: "items",
				Fields:     connectors.Fields("name"),
				Since:      time.Date(2026, 10, 13, 23, 8, 11, 0, time.UTC),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.Path("/v1/items"),
					mockcond.QueryParam("limit", "100"),
					mockcond.QueryParamsMissing("created_at[gt]", "created_at[lte]"),
				},
				Then: mockserver.Response(http.StatusOK, responseItemsAll),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 8,
				Data: []common.ReadResultRow{
					{Id: "QAhcSngCq7Z4YdAw", Fields: map[string]any{"name": "claude-grouppool-cr-20261014221700-item"}, Raw: map[string]any{"id": "QAhcSngCq7Z4YdAw"}},
					{Id: "6DGVHzasyVGJQs5K", Fields: map[string]any{"name": "claude-grouppool-cr-20261014221650-item"}, Raw: map[string]any{"id": "6DGVHzasyVGJQs5K"}},
					{Id: "8HJbXEQ8NixZLxHx", Fields: map[string]any{"name": "claude-grouppool-wo-20261014214429-item"}, Raw: map[string]any{"id": "8HJbXEQ8NixZLxHx"}},
					{Id: "WNCc2Q5TJentgPBY", Fields: map[string]any{"name": "claude-grouppool-20261014214331-item"}, Raw: map[string]any{"id": "WNCc2Q5TJentgPBY"}},
					{Id: "c2m2M5jEQrw5ndJi", Fields: map[string]any{"name": "claude-grouppool-20261014214324-item"}, Raw: map[string]any{"id": "c2m2M5jEQrw5ndJi"}},
					{Id: "PEeEMiz5RbMZmrzd", Fields: map[string]any{"name": "claude-grouppool-20261014214316-item"}, Raw: map[string]any{"id": "PEeEMiz5RbMZmrzd"}},
					{Id: "Tkbo4D7JrUGCwZuf", Fields: map[string]any{"name": "claude-grouppool-20261014214253-item"}, Raw: map[string]any{"id": "Tkbo4D7JrUGCwZuf"}},
					{Id: "B5teGANGLbZCLXRe", Fields: map[string]any{"name": "claude-grouppool-20261014214240-item"}, Raw: map[string]any{"id": "B5teGANGLbZCLXRe"}},
				},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Items stop paging at the first record created before Since",
			Input: common.ReadParams{
				ObjectName: "items",
				Fields:     connectors.Fields("name"),
				Since:      time.Date(2026, 10, 14, 22, 17, 0, 0, time.UTC),
			},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusOK, responseItemsFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id:     "QAhcSngCq7Z4YdAw",
					Fields: map[string]any{"name": "claude-grouppool-cr-20261014221700-item"},
					Raw:    map[string]any{"id": "QAhcSngCq7Z4YdAw"},
				}},
				NextPage: "",
				Done:     true,
			},
		},
		{
			Name: "Items newer than Until are dropped and paging continues",
			Input: common.ReadParams{
				ObjectName: "items",
				Fields:     connectors.Fields("name"),
				Until:      time.Date(2026, 10, 14, 22, 17, 0, 0, time.UTC),
			},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusOK, responseItemsFirstPage),
			}.Server(),
			Comparator: testconn.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Id:     "6DGVHzasyVGJQs5K",
					Fields: map[string]any{"name": "claude-grouppool-cr-20261014221650-item"},
					Raw:    map[string]any{"id": "6DGVHzasyVGJQs5K"},
				}},
				NextPage: "8HJbXEQ8NixZLxHx",
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
