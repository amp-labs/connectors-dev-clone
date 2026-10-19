package main

import (
	"context"
	"log"
	"os"

	ap "github.com/amp-labs/connectors/providers/orb"
	"github.com/amp-labs/connectors/test/orb"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	ctx := context.Background()

	conn := orb.GetOrbConnector(ctx)

	m, err := conn.ListObjectMetadata(ctx, ap.SupportedReadObjects())
	if err != nil {
		log.Fatal(err)
	}

	utils.DumpJSON(m, os.Stdout)
}
