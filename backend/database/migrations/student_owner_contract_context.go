package migrations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type studentContractEvidenceKey struct{}

type studentContractRequest struct {
	evidence StudentContractEvidence
	release  string
}

// WithStudentContractEvidence carries the same reviewed evidence into preflight
// and execution when an operator explicitly supplies it. Ordinary deployments
// use live data checks without requiring an observation document.
func WithStudentContractEvidence(ctx context.Context, evidence StudentContractEvidence, release string) context.Context {
	return context.WithValue(ctx, studentContractEvidenceKey{}, studentContractRequest{evidence: evidence, release: release})
}

type freshStudentStorageKey struct{}

// Only the runner can mark an initial, empty replay. An empty archive on an
// existing installation is not sufficient to skip live data checks.
func studentContractRunContext(ctx context.Context, db *bun.DB) (context.Context, error) {
	var fresh bool
	err := db.NewRaw(`SELECT NOT EXISTS (SELECT 1 FROM public.bun_migrations)
		AND NOT EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'users' AND c.relkind IN ('r', 'p', 'v', 'm', 'f'))`).Scan(ctx, &fresh)
	if err != nil {
		return nil, fmt.Errorf("student contract: inspect initial migration state: %w", err)
	}
	return context.WithValue(ctx, freshStudentStorageKey{}, fresh), nil
}

func studentOwnerContractPrecondition(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return nil // The replay has not created the student schema yet.
	}
	request, ok := ctx.Value(studentContractEvidenceKey{}).(studentContractRequest)
	if !ok {
		return studentOwnerContractDataPreflight(ctx, db)
	}
	if err := validateStudentContractLiveEvidence(ctx, db, request.evidence, studentContractOperatingPolicy(), request.release, time.Now().UTC()); err != nil {
		return err
	}
	return studentOwnerContractDataPreflight(ctx, db)
}

func studentOwnerContractUp(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return contractStudentOwnerStorageChecked(ctx, db, func(ctx context.Context, connection bun.IDB) error {
			var occupied bool
			err := connection.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.students_legacy)
				OR EXISTS (SELECT 1 FROM users.student_profiles)
				OR EXISTS (SELECT 1 FROM users.student_school_memberships)
				OR EXISTS (SELECT 1 FROM users.student_care_profiles)`).Scan(ctx, &occupied)
			if err != nil {
				return fmt.Errorf("student contract: verify empty initial replay: %w", err)
			}
			if occupied {
				return errors.New("student contract: initial replay contains student data; operational evidence is required")
			}
			return nil
		})
	}
	request, ok := ctx.Value(studentContractEvidenceKey{}).(studentContractRequest)
	if !ok {
		// The deployment takes its release backup before this call. The core
		// repeats all live data checks under locks before replacing the stored
		// function and removing compatibility storage in one transaction.
		return contractStudentOwnerStorageChecked(ctx, db, nil)
	}
	return contractStudentOwnerStorageWithEvidence(ctx, db, request.evidence, studentContractOperatingPolicy(), request.release)
}

func studentContractOperatingPolicy() StudentContractPolicy {
	// Retain validation for explicitly supplied historical evidence documents.
	// This optional policy does not block ordinary evidence-free deployments.
	return StudentContractPolicy{
		MinimumRollbackWindow: 24 * time.Hour,
		RequireSchoolDay:      true,
		MaximumEvidenceAge:    24 * time.Hour,
		MaximumBackupAge:      24 * time.Hour,
	}
}
