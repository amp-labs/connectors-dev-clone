package metadata

import (
	_ "embed"

	"github.com/amp-labs/connectors/internal/objectcatalog"
)

var (
	// objectsMetadata is written from the 200 response examples of Orb's list endpoints
	// (https://docs.withorb.com/api-reference), since no OpenAPI spec was available to this build.
	// Subscriptions are absent: the docs carry no subscription schema, so they are sampled live.
	//
	//go:embed objectsMetadata.json
	objectsMetadata []byte

	Catalog = objectcatalog.MustLoad(objectsMetadata) // nolint:gochecknoglobals
)
