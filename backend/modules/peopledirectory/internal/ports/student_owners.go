package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentOwners are the reversible membership/care commands coordinated with
// profile changes inside the caller's unit of work.
type StudentOwners interface {
	Enroll(context.Context, domain.StudentRecord) (int64, error)
	Renew(context.Context, domain.StudentRecord) (int64, error)
	SaveCare(context.Context, int64, domain.StudentRecord, domain.DeparturePlan, *string, bool) error
}
