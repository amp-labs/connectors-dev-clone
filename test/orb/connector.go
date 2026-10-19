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

	client, err := common.NewApiKeyHeaderAuthHTTPClient(ctx,
		"Authorization", "Bearer "+reader.Get(credscanning.Fields.ApiKey))
	if err != nil {
		utils.Fail("error creating Orb client", "error", err)
	}

	conn, err := orb.NewConnector(common.ConnectorParams{AuthenticatedClient: client})
	if err != nil {
		utils.Fail("error creating Orb connector", "error", err)
	}

	return conn
}
