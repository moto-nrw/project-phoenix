package base

import (
	"errors"
	"fmt"
	"testing"
)

const testUniqueConstraint = "idx_guardian_profiles_tenant_email"

func TestIsUniqueViolationOn_DegradedTextError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf(
		"repository create failed: %w",
		errors.New(`ERROR: duplicate key value violates unique constraint "idx_guardian_profiles_tenant_email" (SQLSTATE=23505)`),
	)

	if !IsUniqueViolation(err) {
		t.Fatal("IsUniqueViolation() = false, want true for a degraded textual unique violation")
	}
	if !IsUniqueViolationOn(err, testUniqueConstraint) {
		t.Fatal("IsUniqueViolationOn() = false, want true for the matching textual constraint")
	}
	if IsUniqueViolationOn(err, "another_constraint") {
		t.Fatal("IsUniqueViolationOn() = true for a different textual constraint")
	}
	if IsUniqueViolationOn(err, "idx_guardian_profiles_tenant") {
		t.Fatal("IsUniqueViolationOn() = true for a constraint-name prefix")
	}
}

func TestIsUniqueViolation_NonUniqueErrorsRemainDistinct(t *testing.T) {
	t.Parallel()

	textual := errors.New(`ERROR: insert violates foreign key constraint "idx_guardian_profiles_tenant_email" (SQLSTATE=23503)`)
	plain := errors.New("guardian profile write failed")

	for name, err := range map[string]error{
		"textual foreign-key violation": textual,
		"plain database failure":        plain,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if IsUniqueViolation(err) {
				t.Fatalf("IsUniqueViolation(%v) = true, want false", err)
			}
			if IsUniqueViolationOn(err, testUniqueConstraint) {
				t.Fatalf("IsUniqueViolationOn(%v) = true, want false", err)
			}
		})
	}
}
