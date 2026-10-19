package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	ap "github.com/amp-labs/connectors/providers/orb"
	"github.com/amp-labs/connectors/test/orb"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer done()

	utils.SetupLogging()

	conn := orb.GetOrbConnector(ctx)

	for _, objectName := range ap.SupportedReadObjects() {
		res, err := conn.Read(ctx, common.ReadParams{
			ObjectName: objectName,
			Fields:     connectors.Fields("id", "created_at"),
		})
		if err != nil {
			slog.Error("read failed", "object", objectName, "error", err)

			continue
		}

		slog.Info("read", "object", objectName, "rows", res.Rows, "done", res.Done)
		utils.DumpJSON(res, os.Stdout)
	}
}
