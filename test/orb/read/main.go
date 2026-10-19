package main

import (
	"context"
	"os"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/providers/orb"
	testOrb "github.com/amp-labs/connectors/test/orb"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	ctx := context.Background()

	conn := testOrb.GetOrbConnector(ctx)

	for _, objectName := range orb.SupportedReadObjects() {
		res, err := conn.Read(ctx, common.ReadParams{
			ObjectName: objectName,
			Fields:     connectors.Fields("id", "created_at"),
			PageSize:   5,
		})
		if err != nil {
			utils.Fail("error reading from Orb", "object", objectName, "error", err)
		}

		utils.DumpJSON(res, os.Stdout)
	}
}
