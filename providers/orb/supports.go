package orb

import (
	"fmt"
	"slices"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/components"
)

// Each object is read from GET /v1/{object}, named after its path.
// https://docs.withorb.com/api-reference/customer/list-customers
// https://docs.withorb.com/api-reference/subscription/list-subscriptions
// https://docs.withorb.com/api-reference/invoice/list-invoices
// https://docs.withorb.com/api-reference/plan/list-plans
// https://docs.withorb.com/api-reference/item/list-items
// https://docs.withorb.com/api-reference/credit-note/list-credit-notes
var supportedObjects = []string{ // nolint:gochecknoglobals
	"customers",
	"subscriptions",
	"invoices",
	"plans",
	"items",
	"credit_notes",
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
