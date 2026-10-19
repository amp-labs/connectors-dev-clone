package orb

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

	responseCustomers := testutils.DataFromFile(t, "customers-first-page.json")
	responseSubscriptions := testutils.DataFromFile(t, "subscriptions-first-page.json")

	tests := []testconn.TestCaseListObjectMetadata{
		{
			Name:         "At least one object name must be queried",
			Input:        nil,
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrMissingObjects},
		},
		{
			Name:       "Unknown object is reported per object",
			Input:      []string{"coupons"},
			Server:     mockserver.Dummy(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{},
				Errors: map[string]error{
					"coupons": mockutils.ExpectedSubsetErrors{common.ErrObjectNotSupported},
				},
			},
		},
		{
			Name:  "Fields are sampled from the list endpoints",
			Input: []string{"customers", "subscriptions"},
			Server: mockserver.Switch{
				Setup: mockserver.ContentJSON(),
				Cases: []mockserver.Case{{
					If: mockcond.And{
						mockcond.Path("/v1/customers"),
						mockcond.QueryParam("limit", "20"),
					},
					Then: mockserver.Response(http.StatusOK, responseCustomers),
				}, {
					If: mockcond.And{
						mockcond.Path("/v1/subscriptions"),
						mockcond.QueryParam("limit", "20"),
					},
					Then: mockserver.Response(http.StatusOK, responseSubscriptions),
				}},
			}.Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"customers": {
						DisplayName: "Customers",
						Fields: map[string]common.FieldMetadata{
							"id":              {DisplayName: "Id", ValueType: common.ValueTypeString},
							"auto_collection": {DisplayName: "Auto Collection", ValueType: common.ValueTypeBoolean},
							"metadata":        {DisplayName: "Metadata", ValueType: common.ValueTypeOther},
							// Null in the first record, a string in the second.
							"external_customer_id": {DisplayName: "External Customer Id", ValueType: common.ValueTypeString},
							// Null in every sampled record.
							"currency": {DisplayName: "Currency", ValueType: common.ValueTypeOther},
						},
					},
					"subscriptions": {
						DisplayName: "Subscriptions",
						Fields: map[string]common.FieldMetadata{
							"status":            {DisplayName: "Status", ValueType: common.ValueTypeString},
							"billing_cycle_day": {DisplayName: "Billing Cycle Day", ValueType: common.ValueTypeFloat},
							"plan":              {DisplayName: "Plan", ValueType: common.ValueTypeOther},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:  "Provider error is reported per object",
			Input: []string{"invoices"},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnauthorized),
			}.Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{},
				Errors: map[string]error{
					"invoices": mockutils.ExpectedSubsetErrors{common.ErrAccessToken},
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
