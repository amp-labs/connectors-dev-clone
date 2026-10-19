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
			Name:       "Objects come from the catalog with typed fields",
			Input:      []string{"invoices", "credit_notes", "customers"},
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
								ProviderType: "string",
							},
							"issued_at": {
								DisplayName:  "Issued At",
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
							"customer": {
								DisplayName:  "Customer",
								ValueType:    common.ValueTypeOther,
								ProviderType: "object",
							},
						},
					},
					"credit_notes": {
						DisplayName: "Credit Notes",
						Fields: map[string]common.FieldMetadata{
							"reason": {
								DisplayName:  "Reason",
								ValueType:    common.ValueTypeSingleSelect,
								ProviderType: "string",
								Values: []common.FieldValue{
									{Value: "Duplicate", DisplayValue: "Duplicate"},
									{Value: "Fraudulent", DisplayValue: "Fraudulent"},
									{Value: "Order change", DisplayValue: "Order change"},
									{Value: "Product unsatisfactory", DisplayValue: "Product unsatisfactory"},
								},
							},
						},
					},
					"customers": {
						DisplayName: "Customers",
						Fields: map[string]common.FieldMetadata{
							"external_customer_id": {
								DisplayName:  "External Customer Id",
								ValueType:    common.ValueTypeString,
								ProviderType: "string",
							},
							"auto_issuance": {
								DisplayName:  "Auto Issuance",
								ValueType:    common.ValueTypeBoolean,
								ProviderType: "boolean",
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
