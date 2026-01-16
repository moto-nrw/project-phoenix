package jwt

import (
	"errors"
	"fmt"
)

const errMissingClaim = "missing required claim: %s"

func getRequiredInt(claims map[string]any, key string) (int, error) {
	val, ok := claims[key]
	if !ok {
		return 0, fmt.Errorf(errMissingClaim, key)
	}
	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("claim %s is not a number", key)
	}
	return int(f), nil
}

func getRequiredString(claims map[string]any, key string) (string, error) {
	val, ok := claims[key]
	if !ok {
		return "", fmt.Errorf(errMissingClaim, key)
	}
	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("claim %s is not a string", key)
	}
	return s, nil
}

func getOptionalString(claims map[string]any, key string) string {
	val, ok := claims[key]
	if !ok || val == nil {
		return ""
	}
	s, _ := val.(string)
	return s
}

func getOptionalBool(claims map[string]any, key string) bool {
	val, ok := claims[key]
	if !ok || val == nil {
		return false
	}
	b, _ := val.(bool)
	return b
}

func getRequiredStringSlice(claims map[string]any, key string) ([]string, error) {
	val, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf(errMissingClaim, key)
	}
	result, err := toStringSliceStrict(val)
	if err != nil {
		return nil, fmt.Errorf("claim %s: %w", key, err)
	}
	return result, nil
}

func getOptionalStringSlice(claims map[string]any, key string) []string {
	val, ok := claims[key]
	if !ok || val == nil {
		return []string{}
	}
	return toStringSliceLenient(val)
}

func toStringSliceStrict(val any) ([]string, error) {
	slice, ok := val.([]any)
	if !ok {
		return nil, errors.New("value is not an array")
	}
	result := make([]string, 0, len(slice))
	for i, v := range slice {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("element %d is not a string", i)
		}
		result = append(result, s)
	}
	return result, nil
}

func toStringSliceLenient(val any) []string {
	slice, ok := val.([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
