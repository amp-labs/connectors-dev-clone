package clarify

import (
	"fmt"
	"slices"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/components"
	"github.com/amp-labs/connectors/internal/datautils"
)

const (
	objectNamePerson  = "person"
	objectNameCompany = "company"
	objectNameDeal    = "deal"
	objectNameMeeting = "meeting"
	objectNameTask    = "task"
	// Workspace users, listed by GET /v1/workspaces/{workspace}/users. Their JSON:API type is `user`.
	objectNameUser = "user"
)

var supportedObjects = []string{ // nolint:gochecknoglobals
	objectNamePerson,
	objectNameCompany,
	objectNameDeal,
	objectNameMeeting,
	objectNameTask,
	objectNameUser,
}

// Built-in objects read through GET /v1/workspaces/{workspace}/objects/{object}/resources and
// described by GET /v1/workspaces/{workspace}/schemas.
// https://developer.clarify.ai/docs/getting-started/key-concepts#objects
var entityObjects = datautils.NewSet( // nolint:gochecknoglobals
	objectNamePerson,
	objectNameCompany,
	objectNameDeal,
	objectNameMeeting,
	objectNameTask,
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
