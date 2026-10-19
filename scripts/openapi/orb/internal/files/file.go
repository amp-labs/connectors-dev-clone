package files

import (
	_ "embed"

	"github.com/amp-labs/connectors/scripts/openapi/internal/api"
)

var (
	// Orb's OpenAPI spec, as cited by its docs: https://docs.withorb.com/api-reference/orb-openapi.json
	//
	//go:embed openapi.json
	apiFile []byte

	OpenApiFile = api.NewFile(apiFile)              // nolint:gochecknoglobals
	OutputDir   = "providers/orb/internal/metadata" // nolint:gochecknoglobals
)
