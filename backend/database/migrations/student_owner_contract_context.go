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
// and execution. It does not validate or authorize it. Operating policy belongs
// to the migration, not the evidence file or a caller-controlled CLI flag.
func WithStudentContractEvidence(ctx context.Context, evidence StudentContractEvidence, release string) context.Context {
	return context.WithValue(ctx, studentContractEvidenceKey{}, studentContractRequest{evidence: evidence, release: release})
}

type freshStudentStorageKey struct{}

// Only the runner can mark an initial, empty replay. An empty archive on an
// existing installation is not sufficient to bypass operational evidence.
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
		return errors.New("student contract: reviewed --student-contract-evidence file is required")
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
		return errors.New("student contract: reviewed --student-contract-evidence file is required")
	}
	return contractStudentOwnerStorageWithEvidence(ctx, db, request.evidence, studentContractOperatingPolicy(), request.release)
}

func studentContractOperatingPolicy() StudentContractPolicy {
	// User decision for #2760: at least 24 hours, including one complete
	// regular school day and relevant jobs without old access. Freshness caps
	// are not additional waiting periods; live state is rechecked before DDL.
	return StudentContractPolicy{
		MinimumRollbackWindow: 24 * time.Hour,
		RequireSchoolDay:      true,
		MaximumEvidenceAge:    24 * time.Hour,
		MaximumBackupAge:      24 * time.Hour,
	}
}
