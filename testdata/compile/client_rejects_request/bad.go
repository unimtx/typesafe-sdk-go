package bad

import (
	"github.com/unimtx/typesafe-sdk-go"
	"github.com/unimtx/typesafe-sdk-go/option"
	"net/http"
)

func bad() {
	var response *http.Response
	_, _ = typesafe.NewClient(option.WithResponseInto(&response))
}
