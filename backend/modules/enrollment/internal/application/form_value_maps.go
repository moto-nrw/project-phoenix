package application

import (
	"maps"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The form logic of the intake and the change requests validates, filters
// and merges answers as maps. The request and its children carry them as
// enrollment.FormValues (ADR 0170); these helpers copy between the two.

// formMap copies values into a new, never nil map the caller may change.
func formMap(values enrollment.FormValues) map[string]any {
	return maps.Collect(values.All())
}

// formValues copies a working map into the values a request stores.
func formValues(values map[string]any) enrollment.FormValues {
	return enrollment.CollectFormValues(maps.All(values))
}

// formValue is the value stored under key, nil when absent.
func formValue(values enrollment.FormValues, key string) any {
	value, _ := values.Lookup(key)
	return value
}
