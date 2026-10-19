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

	responseSubscriptions := testutils.DataFromFile(t, "subscriptions-sample.json")
	responseSubscriptionsEmpty := testutils.DataFromFile(t, "subscriptions-empty.json")
	responseUnauthorized := testutils.DataFromFile(t, "error-unauthorized.json")

	tests := []testconn.TestCaseListObjectMetadata{
		{
			Name:         "At least one object name must be queried",
			Input:        nil,
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrMissingObjects},
		},
		{
			Name:       "Unknown object is reported per object",
			Input:      []string{"prices"},
			Server:     mockserver.Dummy(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{},
				Errors: map[string]error{
					"prices": mockutils.ExpectedSubsetErrors{common.ErrObjectNotSupported},
				},
			},
		},
		{
			Name:       "Invoices and credit notes come from the static catalog",
			Input:      []string{"invoices", "credit_notes"},
			Server:     mockserver.Dummy(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"invoices": {
						DisplayName: "Invoices",
						Fields: map[string]common.FieldMetadata{
							"id": {
								DisplayName:  "Id",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
							"invoice_date": {
								DisplayName:  "Invoice Date",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "string<date-time>",
							},
							"hidden_line_item_count": {
								DisplayName:  "Hidden Line Item Count",
								ValueType:    common.ValueTypeInt,
								ProviderType: "integer",
							},
							"will_auto_issue": {
								DisplayName:  "Will Auto Issue",
								ValueType:    common.ValueTypeBoolean,
								ProviderType: "boolean",
							},
							"customer": {
								DisplayName:  "Customer",
								ValueType:    common.ValueTypeOther,
								ProviderType: "object",
							},
							"status": {
								DisplayName:  "Status",
								ValueType:    common.ValueTypeSingleSelect,
								ProviderType: "enum<string>",
								Values: []common.FieldValue{
									{Value: "draft", DisplayValue: "Draft"},
									{Value: "issued", DisplayValue: "Issued"},
									{Value: "paid", DisplayValue: "Paid"},
									{Value: "synced", DisplayValue: "Synced"},
									{Value: "void", DisplayValue: "Void"},
								},
							},
						},
					},
					"credit_notes": {
						DisplayName: "Credit Notes",
						Fields: map[string]common.FieldMetadata{
							"credit_note_number": {
								DisplayName:  "Credit Note Number",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
							"line_items": {
								DisplayName:  "Line Items",
								ValueType:    common.ValueTypeOther,
								ProviderType: "array",
							},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:  "Subscriptions are sampled from one live record",
			Input: []string{"subscriptions", "coupons"},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1/subscriptions"),
					mockcond.QueryParam("limit", "1"),
				},
				Then: mockserver.Response(http.StatusOK, responseSubscriptions),
			}.Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"subscriptions": {
						DisplayName: "Subscriptions",
						Fields: map[string]common.FieldMetadata{
							"id":         {DisplayName: "Id", ValueType: common.ValueTypeString},
							"created_at": {DisplayName: "Created At", ValueType: common.ValueTypeString},
							"net_terms":  {DisplayName: "Net Terms", ValueType: common.ValueTypeFloat},
							"plan":       {DisplayName: "Plan", ValueType: common.ValueTypeOther},
							"status":     {DisplayName: "Status", ValueType: common.ValueTypeString},
							"auto_collection": {
								DisplayName: "Auto Collection",
								ValueType:   common.ValueTypeBoolean,
							},
						},
					},
					"coupons": {
						DisplayName: "Coupons",
						Fields: map[string]common.FieldMetadata{
							"redemption_code": {
								DisplayName:  "Redemption Code",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:  "Subscriptions without a record to sample are reported per object",
			Input: []string{"subscriptions", "items"},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusOK, responseSubscriptionsEmpty),
			}.Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"items": {
						DisplayName: "Items",
						Fields: map[string]common.FieldMetadata{
							"archived_at": {
								DisplayName:  "Archived At",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "string<date-time>",
							},
						},
					},
				},
				Errors: map[string]error{
					"subscriptions": mockutils.ExpectedSubsetErrors{common.ErrMissingExpectedValues},
				},
			},
		},
		{
			Name:  "Provider error while sampling subscriptions is reported per object",
			Input: []string{"subscriptions"},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnauthorized, responseUnauthorized),
			}.Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{},
				Errors: map[string]error{
					"subscriptions": mockutils.ExpectedSubsetErrors{common.ErrAccessToken},
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
