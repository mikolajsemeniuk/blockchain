package blockchain

import _ "embed"

//go:embed templates/openapi.yaml
var docs string

//go:embed templates/elements.tpl
var elements string
