package utilities

import (
	"encoding/base64"
	"fmt"
)

func Base64Encode(v string) string {
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func Base64Decode(v string) string {
	data, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		panic(fmt.Sprintf("Error decoding base64: %v", err))
	}
	return string(data)
}
