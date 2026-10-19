package clarify

import (
	"fmt"
	"slices"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/components"
)

// Built-in Clarify objects. Each is listed by GET /v1/workspaces/{workspace}/objects/{object}/resources
// and described by the `entities/{object}` schema of GET /v1/workspaces/{workspace}/schemas.
// `user` is an entity in the `objects` namespace too, so it is read like the others rather than through
// GET /users, which takes no filter.
// https://developer.clarify.ai/docs/getting-started/key-concepts#objects
var supportedObjects = []string{ // nolint:gochecknoglobals
	"person",
	"company",
	"deal",
	"meeting",
	"task",
	"user",
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
