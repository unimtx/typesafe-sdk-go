package bad

import "github.com/unimtx/typesafe-sdk-go"

var _ = typesafe.Score("?", "low", map[string]any{"level": "high"})
