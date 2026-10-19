package orb

import (
	"fmt"
	"slices"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/components"
)

const (
	objectNameCustomers     = "customers"
	objectNameSubscriptions = "subscriptions"
	objectNameInvoices      = "invoices"
	objectNamePlans         = "plans"
	objectNameItems         = "items"
	objectNameCreditNotes   = "credit_notes"
)

var supportedObjects = []string{ // nolint:gochecknoglobals
	objectNameCustomers,
	objectNameSubscriptions,
	objectNameInvoices,
	objectNamePlans,
	objectNameItems,
	objectNameCreditNotes,
}

// Orb records carry no updated time, so Since and Until bound the time each list endpoint can filter on.
// Customers, subscriptions, plans and credit notes accept created_at[gt|gte|lt|lte];
// invoices accept invoice_date[gt|gte|lt|lte]; items accept no filter and are filtered on created_at
// by the connector (see parseReadResponse).
// https://docs.withorb.com/api-reference/customer/list-customers
// https://docs.withorb.com/api-reference/invoice/list-invoices
// https://docs.withorb.com/api-reference/item/list-items
var timeFilterField = map[string]string{ // nolint:gochecknoglobals
	objectNameCustomers:     "created_at",
	objectNameSubscriptions: "created_at",
	objectNamePlans:         "created_at",
	objectNameCreditNotes:   "created_at",
	objectNameInvoices:      "invoice_date",
}

// SupportedReadObjects lists the objects this connector reads.
func SupportedReadObjects() []string {
	return slices.Clone(supportedObjects)
}

func supportedOperations() components.EndpointRegistryInput {
	return components.EndpointRegistryInput{
		common.ModuleRoot: {
			{
				Endpoint: fmt.Sprintf("{%s}", strings.Join(supportedObjects, ",")),
				Support:  components.ReadSupport,
			},
		},
	}
}
