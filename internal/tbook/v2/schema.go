package v2

import (
	_ "embed"
)

// SchemaJSON is the published JSON Schema every file ships byte-identical at
// manifest.schema (spec §3.9).
//
//go:embed schema/tbook-2.schema.json
var SchemaJSON []byte
