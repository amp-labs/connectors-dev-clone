package lightfield

import (
	"fmt"
	"slices"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/components"
	"github.com/amp-labs/connectors/internal/datautils"
)

const (
	objectNameAccounts      = "accounts"
	objectNameContacts      = "contacts"
	objectNameOpportunities = "opportunities"
	// Workspace members (GET /v1/members); the customer's "users".
	objectNameMembers = "members"
	objectNameNotes   = "notes"
	objectNameTasks   = "tasks"
)

var supportedObjects = []string{ // nolint:gochecknoglobals
	objectNameAccounts,
	objectNameContacts,
	objectNameOpportunities,
	objectNameMembers,
	objectNameNotes,
	objectNameTasks,
}

// Objects that expose GET /v1/{object}/definitions.
// https://docs.lightfield.app/using-the-api/fields-and-relationships#supported-object-types
// That guide omits notes and tasks; their definitions endpoints are in the API reference:
// https://docs.lightfield.app/api/resources/note/methods/definitions
// https://docs.lightfield.app/api/resources/task/methods/definitions
var objectsWithDefinitions = datautils.NewSet( // nolint:gochecknoglobals
	objectNameAccounts,
	objectNameContacts,
	objectNameOpportunities,
	objectNameNotes,
	objectNameTasks,
)

// Objects whose list endpoint filters on `$updatedAt[greaterThan|lessThan]`.
// /v1/members documents only limit and offset and rejects filters (400 unrecognized_keys).
// https://docs.lightfield.app/using-the-api/list-endpoints#filtering
// TODO: notes and tasks are filtered by the connector. Unlike accounts, contacts and opportunities,
// their list references do not offer `$field` filters, as with members; the filtering guide does
// show `$status[equal]` on /v1/tasks. A live Since read would settle whether they accept `$updatedAt`.
// https://docs.lightfield.app/api/resources/note/methods/list
// https://docs.lightfield.app/api/resources/task/methods/list
var objectsWithUpdatedAtFilter = datautils.NewSet( // nolint:gochecknoglobals
	objectNameAccounts,
	objectNameContacts,
	objectNameOpportunities,
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
