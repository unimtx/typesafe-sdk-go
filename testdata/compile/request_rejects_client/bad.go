package bad

import (
	"context"
	"github.com/unimtx/typesafe-sdk-go"
	"github.com/unimtx/typesafe-sdk-go/option"
)

func bad(c *typesafe.Client) {
	_, _ = c.SystemOne(context.Background(), typesafe.SystemOneRequest{}, option.WithAPIKey("no"))
}
