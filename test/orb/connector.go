package orb

import (
	"context"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/scanning/credscanning"
	"github.com/amp-labs/connectors/providers"
	"github.com/amp-labs/connectors/providers/orb"
	"github.com/amp-labs/connectors/test/utils"
)

func GetOrbConnector(ctx context.Context) *orb.Connector {
	filePath := credscanning.LoadPath(providers.Orb)
	reader := utils.MustCreateProvCredJSON(filePath, false)

	client := utils.NewAPIKeyClient(ctx, reader, providers.Orb)

	conn, err := orb.NewConnector(common.ConnectorParams{
		AuthenticatedClient: client,
	})
	if err != nil {
		utils.Fail("error creating orb connector", "error", err)
	}

	return conn
}
