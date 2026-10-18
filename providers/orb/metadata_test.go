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
	responseEmpty := testutils.DataFromFile(t, "empty-page.json")
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
			Input:      []string{"events"},
			Server:     mockserver.Dummy(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{},
				Errors: map[string]error{
					"events": mockutils.ExpectedSubsetErrors{common.ErrObjectNotSupported},
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
							"status": {
								DisplayName:  "Status",
								ValueType:    common.ValueTypeSingleSelect,
								ProviderType: "enum",
								Values: []common.FieldValue{
									{Value: "draft", DisplayValue: "Draft"},
									{Value: "issued", DisplayValue: "Issued"},
									{Value: "paid", DisplayValue: "Paid"},
									{Value: "synced", DisplayValue: "Synced"},
									{Value: "void", DisplayValue: "Void"},
								},
							},
							"paid_at": {
								DisplayName:  "Paid At",
								ValueType:    common.ValueTypeDateTime,
								ProviderType: "string",
							},
							"line_items": {
								DisplayName:  "Line Items",
								ValueType:    common.ValueTypeOther,
								ProviderType: "array",
							},
						},
					},
					"credit_notes": {
						DisplayName: "Credit Notes",
						Fields: map[string]common.FieldMetadata{
							"invoice_id": {
								DisplayName:  "Invoice Id",
								ValueType:    common.ValueTypeReference,
								ProviderType: "string",
								ReferenceTo:  []string{"invoices"},
							},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:  "Subscriptions are sampled from the newest record",
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
							"id":              {DisplayName: "Id", ValueType: common.ValueTypeString},
							"start_date":      {DisplayName: "Start Date", ValueType: common.ValueTypeString},
							"net_terms":       {DisplayName: "Net Terms", ValueType: common.ValueTypeFloat},
							"auto_collection": {DisplayName: "Auto Collection", ValueType: common.ValueTypeBoolean},
							"plan":            {DisplayName: "Plan", ValueType: common.ValueTypeOther},
						},
					},
					"coupons": {
						DisplayName: "Coupons",
						Fields: map[string]common.FieldMetadata{
							"times_redeemed": {
								DisplayName:  "Times Redeemed",
								ValueType:    common.ValueTypeInt,
								ProviderType: "integer",
							},
						},
					},
				},
				Errors: map[string]error{},
			},
		},
		{
			Name:  "Subscriptions cannot be sampled from an empty account",
			Input: []string{"subscriptions"},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusOK, responseEmpty),
			}.Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{},
				Errors: map[string]error{
					"subscriptions": mockutils.ExpectedSubsetErrors{common.ErrMissingExpectedValues},
				},
			},
		},
		{
			Name:  "Provider error while sampling subscriptions is reported per object",
			Input: []string{"subscriptions", "items"},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnauthorized, responseUnauthorized),
			}.Server(),
			Comparator: testconn.ComparatorSubsetMetadata,
			Expected: &common.ListObjectMetadataResult{
				Result: map[string]common.ObjectMetadata{
					"items": {
						DisplayName: "Items",
						Fields: map[string]common.FieldMetadata{
							"name": {DisplayName: "Name", ValueType: common.ValueTypeString, ProviderType: "string"},
						},
					},
				},
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
