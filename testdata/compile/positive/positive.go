package positive

import (
	"context"
	"net/http"
	"time"

	"github.com/unimtx/typesafe-sdk-go"
	"github.com/unimtx/typesafe-sdk-go/option"
)

func compile() {
	common := option.WithTimeout(time.Second)
	clientOptions := []option.ClientOption{option.WithAPIKey("test-key"), common}
	client, _ := typesafe.NewClient(clientOptions...)
	var raw *http.Response
	requestOptions := []option.RequestOption{common, option.WithResponseInto(&raw)}
	_, _ = client.SystemOne(context.Background(), typesafe.SystemOneRequest{State: "s", Questions: typesafe.Questions{
		"typed": typesafe.Score("?", []string{"low", "high"}...),
		"mixed": typesafe.Score[typesafe.Entry]("?", "low", map[string]any{"level": "high"}),
	}}, requestOptions...)
}
