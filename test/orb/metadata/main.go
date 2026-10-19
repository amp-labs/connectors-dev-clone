package main

import (
	"context"
	"os"

	"github.com/amp-labs/connectors/providers/orb"
	testOrb "github.com/amp-labs/connectors/test/orb"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	ctx := context.Background()

	conn := testOrb.GetOrbConnector(ctx)

	metadata, err := conn.ListObjectMetadata(ctx, orb.SupportedReadObjects())
	if err != nil {
		utils.Fail("error listing metadata for Orb", "error", err)
	}

	utils.DumpJSON(metadata, os.Stdout)
}
