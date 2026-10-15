package compose

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/uptrace/bun"
)

// VerifyRetainedStudentSchema runs the startup check of the students table:
// the server starts only against a fully migrated schema (#2059).
func VerifyRetainedStudentSchema(ctx context.Context, db *bun.DB) error {
	return usersRepo.VerifyStudentSchema(ctx, db)
}
