package orb

import (
	"fmt"
	"slices"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/components"
	"github.com/amp-labs/connectors/internal/datautils"
)

const (
	objectNameCustomers     = "customers"
	objectNameSubscriptions = "subscriptions"
	objectNameInvoices      = "invoices"
	objectNamePlans         = "plans"
	objectNameItems         = "items"
	objectNameCreditNotes   = "credit_notes"
	objectNameCoupons       = "coupons"
)

// Each object is read from GET /v1/{object}.
var supportedObjects = []string{ // nolint:gochecknoglobals
	objectNameCustomers,
	objectNameSubscriptions,
	objectNameInvoices,
	objectNamePlans,
	objectNameItems,
	objectNameCreditNotes,
	objectNameCoupons,
}

// Objects whose list endpoint filters on `created_at[gt|lte]`. Orb records carry no updated
// timestamp, so creation time is the only server-side filter there is.
// Customers, plans and credit notes document it; subscriptions accept it too (verified live).
// Invoices filter only on invoice_date and due_date, and items and coupons have no time filter,
// so those three are always read in full.
// https://docs.withorb.com/api-reference/customer/list-customers
var objectsWithCreatedAtFilter = datautils.NewSet( // nolint:gochecknoglobals
	objectNameCustomers,
	objectNameSubscriptions,
	objectNamePlans,
	objectNameCreditNotes,
)

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
