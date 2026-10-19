package main

import (
	"context"
	"log"
	"os"

	ap "github.com/amp-labs/connectors/providers/clarify"
	"github.com/amp-labs/connectors/test/clarify"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	ctx := context.Background()

	conn := clarify.GetClarifyConnector(ctx)

	m, err := conn.ListObjectMetadata(ctx, ap.SupportedReadObjects())
	if err != nil {
		log.Fatal(err)
	}

	utils.DumpJSON(m, os.Stdout)
}
