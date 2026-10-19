package clarify

import (
	"context"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/scanning/credscanning"
	"github.com/amp-labs/connectors/providers"
	"github.com/amp-labs/connectors/providers/clarify"
	"github.com/amp-labs/connectors/test/utils"
)

var fieldWorkspace = credscanning.Field{ //nolint:gochecknoglobals
	Name:      "workspace",
	PathJSON:  "metadata.workspace",
	SuffixENV: "WORKSPACE",
}

func GetClarifyConnector(ctx context.Context) *clarify.Connector {
	filePath := credscanning.LoadPath(providers.Clarify)
	reader := utils.MustCreateProvCredJSON(filePath, false, fieldWorkspace)

	client := utils.NewAPIKeyClient(ctx, reader, providers.Clarify)

	conn, err := clarify.NewConnector(common.ConnectorParams{
		AuthenticatedClient: client,
		Workspace:           reader.Get(fieldWorkspace),
	})
	if err != nil {
		utils.Fail("error creating Clarify connector", "error", err)
	}

	return conn
}
