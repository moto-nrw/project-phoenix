package common

import (
	"errors"
	"sort"

	validation "github.com/go-ozzo/ozzo-validation"
)

// validationFieldErrors lists the per-field problems of a request Bind that
// failed in ozzo-validation, so the client can mark every affected field
// instead of reading the summary sentence (ADR 0006). The keys are the JSON
// field names (ozzo's ErrorTag is "json"); nested errors, such as one row of
// a list, join their path with dots. Returns nil for any other error.
func validationFieldErrors(err error) []FieldError {
	errs, ok := errors.AsType[validation.Errors](err)
	if !ok {
		return nil
	}
	fields := appendValidationFieldErrors(nil, "", errs)
	sort.Slice(fields, func(i, j int) bool { return fields[i].Field < fields[j].Field })
	return fields
}

func appendValidationFieldErrors(fields []FieldError, prefix string, errs validation.Errors) []FieldError {
	for key, err := range errs {
		if err == nil {
			continue
		}
		field := key
		if prefix != "" {
			field = prefix + "." + key
		}
		if nested, ok := errors.AsType[validation.Errors](err); ok {
			fields = appendValidationFieldErrors(fields, field, nested)
			continue
		}
		fields = append(fields, FieldError{Field: field, Reason: err.Error()})
	}
	return fields
}
