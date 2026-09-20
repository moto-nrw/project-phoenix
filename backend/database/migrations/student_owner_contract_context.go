package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

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
				return errors.New("student contract: initial replay contains student data")
			}
			return nil
		})
	}
	// Deployment stops the old application and verifies a complete release
	// backup first. Recheck integrity under locks before removing old storage.
	return contractStudentOwnerStorageChecked(ctx, db, nil)
}
