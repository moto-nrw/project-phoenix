package api

import "encoding/json"

func parseJSON[T any](body []byte, target *T) error {
	return json.Unmarshal(body, target)
}
