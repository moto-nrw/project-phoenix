package postgres

import (
	"errors"
	"fmt"
	"testing"
)

type fieldError struct{ fields map[byte]string }

func (e fieldError) Error() string           { return "driver error" }
func (e fieldError) Field(field byte) string { return e.fields[field] }

func TestUniqueViolationClassification(t *testing.T) {
	t.Parallel()

	const constraint = "idx_activity_instances_template_unique"
	cases := []struct {
		name     string
		err      error
		unique   bool
		onTarget bool
	}{
		{"structured on the constraint", fmt.Errorf("insert: %w", fieldError{map[byte]string{'C': "23505", 'n': constraint}}), true, true},
		{"structured on another constraint", fieldError{map[byte]string{'C': "23505", 'n': "other"}}, true, false},
		{"structured foreign key", fieldError{map[byte]string{'C': "23503", 'n': constraint}}, false, false},
		{"degraded text", errors.New(`ERROR: duplicate key value violates unique constraint "` + constraint + `" (SQLSTATE=23505)`), true, true},
		{"localized degraded text", errors.New(`FEHLER: doppelter Schlüsselwert verletzt eindeutige Bedingung »` + constraint + `« (SQLSTATE=23505)`), true, true},
		{"identifier only in detail", errors.New(`ERROR: duplicate key value violates unique constraint "other" (SQLSTATE=23505)` + "\n" + `DETAIL: Key (x)=("` + constraint + `") already exists.`), true, false},
		{"constraint name prefix", errors.New(`ERROR: duplicate key value violates unique constraint "` + constraint + `_x" (SQLSTATE=23505)`), true, false},
		{"plain failure", errors.New("connection reset"), false, false},
		{"nil", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsUniqueViolation(tc.err); got != tc.unique {
				t.Fatalf("IsUniqueViolation(%v) = %v, want %v", tc.err, got, tc.unique)
			}
			if got := IsUniqueViolationOn(tc.err, constraint); got != tc.onTarget {
				t.Fatalf("IsUniqueViolationOn(%v) = %v, want %v", tc.err, got, tc.onTarget)
			}
		})
	}
	if IsUniqueViolationOn(fieldError{map[byte]string{'C': "23505", 'n': ""}}, "") {
		t.Fatal("IsUniqueViolationOn with an empty constraint = true, want false")
	}
}
