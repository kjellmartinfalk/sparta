package utilities

import (
	"encoding/json"
	"fmt"
	"strings"
)

func JsonField(jsonStr string, path string) (interface{}, error) {
	val, err := extractJSONField(jsonStr, path)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func MustJsonField(jsonStr string, path string) interface{} {
	val, err := extractJSONField(jsonStr, path)
	if err != nil {
		panic(fmt.Sprintf("Error extracting JSON field: %v", err))
	}
	return val
}

func extractJSONField(jsonStr string, path string) (interface{}, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil, fmt.Errorf("field %s not found", part)
			}
		case []interface{}:
			return nil, fmt.Errorf("array indexing not supported yet")
		default:
			return nil, fmt.Errorf("invalid path: %s", part)
		}
	}

	return current, nil
}
