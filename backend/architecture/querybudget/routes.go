package querybudget

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Routes is one resolved pattern, or the complete sorted set for a scenario
// measuring several requests. JSON retains a scalar for the common single route.
type Routes []string

func (r *Routes) UnmarshalJSON(data []byte) error {
	var routes []string
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte(`"`)) {
		var route string
		if err := json.Unmarshal(data, &route); err != nil {
			return err
		}
		routes = []string{route}
	} else {
		if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
			return fmt.Errorf("query budget route cannot be null")
		}
		if err := json.Unmarshal(data, &routes); err != nil {
			return err
		}
	}
	for _, route := range routes {
		if strings.TrimSpace(route) == "" {
			return fmt.Errorf("query budget route cannot be empty")
		}
	}
	slices.Sort(routes)
	*r = slices.Compact(routes)
	return nil
}

func (r Routes) MarshalJSON() ([]byte, error) {
	if len(r) == 1 {
		return json.Marshal(r[0])
	}
	return json.Marshal([]string(r))
}
