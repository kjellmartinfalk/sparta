package utilities

import (
	"encoding/base64"
	"fmt"

	"github.com/kjellmartinfalk/sparta/functions"
)

func init() {
	functions.RegisterFunction("b64enc", base64Encode)
	functions.RegisterFunction("b64dec", base64Decode)
}

func base64Encode(v string) string {
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func base64Decode(v string) string {
	data, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		panic(fmt.Sprintf("Error decoding base64: %v", err))
	}
	return string(data)
}
