package metadata

import (
	_ "embed"

	"github.com/amp-labs/connectors/internal/objectcatalog"
)

var (
	// objectsMetadata is written from the example list responses in Orb's API reference,
	// e.g. https://docs.withorb.com/api-reference/customer/list-customers. Orb publishes no
	// OpenAPI spec to generate it from. Subscriptions are not in it: their list page is not
	// in the reference, so ListObjectMetadata samples a live record instead.
	//
	//go:embed objectsMetadata.json
	objectsMetadata []byte

	Catalog = objectcatalog.MustLoad(objectsMetadata) // nolint:gochecknoglobals
)
