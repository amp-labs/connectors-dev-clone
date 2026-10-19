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
// That table omits notes and tasks, which have their own definitions methods:
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
var objectsWithUpdatedAtFilter = datautils.NewSet( // nolint:gochecknoglobals
	objectNameAccounts,
	objectNameContacts,
	objectNameOpportunities,
	objectNameNotes,
	objectNameTasks,
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
