package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// JSONID is an int64 request field that accepts a JSON number or a decimal
// string ("42" and 42 both decode to 42).
//
// Database ids are bigints, and JavaScript has no integer type that holds one:
// every id above 2^53 is silently rounded the moment JSON.parse sees it as a
// number, and the rounded value is a perfectly valid id for a different row.
// The frontend therefore carries ids as decimal strings — the project
// convention for int64 ids on the way out (backend int64 to frontend string) —
// and the Next.js proxy routes that parse and re-serialize a body cannot honour
// that convention while the receiving field is number-only.
//
// Accepting both keeps every existing client working: a number decodes exactly
// as it did before.
type JSONID int64

// Int64 returns the id as the int64 the rest of the code works with.
func (id JSONID) Int64() int64 { return int64(id) }

// UnmarshalJSON accepts a JSON number or a decimal string. A string is parsed
// with strconv, so digits beyond float64 precision survive, and anything that
// is not a plain integer is rejected instead of being coerced to zero.
func (id *JSONID) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		return nil
	}

	if strings.HasPrefix(trimmed, `"`) {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return errors.New("id must not be empty")
		}
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid id %q: %w", raw, err)
		}
		*id = JSONID(parsed)
		return nil
	}

	var parsed int64
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*id = JSONID(parsed)
	return nil
}

// MarshalJSON emits a JSON number, so a struct that is decoded and re-encoded
// keeps the shape every existing consumer expects.
func (id JSONID) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(id), 10)), nil
}
