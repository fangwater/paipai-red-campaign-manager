package delivery

import _ "embed"

// OpenAPIJSON is the embedded contract for the delivery HTTP API.
//
//go:embed openapi.json
var OpenAPIJSON []byte
