package orb

import (
	"testing"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockserver"
	"github.com/amp-labs/connectors/test/utils/testconn"
)

func TestListObjectMetadata(t *testing.T) { // nolint:funlen
	t.Parallel()

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
			Name:       "Fields come from the generated catalog without any request",
			Input:      []string{"customers", "invoices", "subscriptions", "credit_notes"},
			Server:     mockserver.Dummy(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"customers": {
						DisplayName: "Customers",
						Fields: map[string]common.FieldMetadata{
							"id": {
								DisplayName:  "Id",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
							"created_at": {
								DisplayName:  "Created At",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "string",
							},
							// Nullable in the spec: oneOf [boolean, null].
							"auto_issuance": {
								DisplayName:  "Auto Issuance",
								ValueType:    common.ValueTypeBoolean,
								ProviderType: "boolean",
							},
							"billing_address": {
								DisplayName:  "Billing Address",
								ValueType:    common.ValueTypeOther,
								ProviderType: "object",
							},
							"payment_provider": {
								DisplayName:  "Payment Provider",
								ValueType:    common.ValueTypeSingleSelect,
								ProviderType: "string",
								Values: []common.FieldValue{
									{Value: "quickbooks", DisplayValue: "quickbooks"},
									{Value: "bill.com", DisplayValue: "bill.com"},
									{Value: "stripe_charge", DisplayValue: "stripe_charge"},
									{Value: "stripe_invoice", DisplayValue: "stripe_invoice"},
									{Value: "netsuite", DisplayValue: "netsuite"},
									{Value: "adyen", DisplayValue: "adyen"},
								},
							},
						},
					},
					"invoices": {
						DisplayName: "Invoices",
						Fields: map[string]common.FieldMetadata{
							"voided_at": {
								DisplayName:  "Voided At",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "string",
							},
							"status": {
								DisplayName:  "Status",
								ValueType:    common.ValueTypeSingleSelect,
								ProviderType: "string",
								Values: []common.FieldValue{
									{Value: "issued", DisplayValue: "issued"},
									{Value: "paid", DisplayValue: "paid"},
									{Value: "synced", DisplayValue: "synced"},
									{Value: "void", DisplayValue: "void"},
									{Value: "draft", DisplayValue: "draft"},
								},
							},
						},
					},
					"subscriptions": {
						DisplayName: "Subscriptions",
						Fields: map[string]common.FieldMetadata{
							"billing_cycle_day": {
								DisplayName:  "Billing Cycle Day",
								ValueType:    common.ValueTypeInt,
								ProviderType: "integer",
							},
						},
					},
					"credit_notes": {
						DisplayName: "Credit Notes",
						Fields: map[string]common.FieldMetadata{
							"total": {
								DisplayName:  "Total",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
						},
					},
				},
				Errors: map[string]error{},
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
