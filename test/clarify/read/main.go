package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	ap "github.com/amp-labs/connectors/providers/clarify"
	"github.com/amp-labs/connectors/test/clarify"
)

func main() {
	os.Exit(MainFn())
}

func MainFn() int {
	ctx := context.Background()
	conn := clarify.GetClarifyConnector(ctx)

	for _, objectName := range ap.SupportedReadObjects() {
		if err := testRead(ctx, conn, objectName, []string{"id", "_updated_at"}); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)

			return 1
		}
	}

	return 0
}

func testRead(ctx context.Context, conn *ap.Connector, objName string, fields []string) error {
	params := common.ReadParams{
		ObjectName: objName,
		Fields:     connectors.Fields(fields...),
		PageSize:   5,
	}

	res, err := conn.Read(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", objName, err)
	}

	jsonStr, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %w", err)
	}

	_, _ = os.Stdout.Write(jsonStr)
	_, _ = os.Stdout.WriteString("\n")

	return nil
}
