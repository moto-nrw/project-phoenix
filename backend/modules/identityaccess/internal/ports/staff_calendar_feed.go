package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type StaffCalendarFeedStore interface {
	FindStaffCalendarFeedOwner(context.Context, string) (domain.StaffCalendarFeedOwner, bool, domain.OperationStats, error)
	EnsureStaffCalendarFeedToken(context.Context, int64, int64, string) (string, domain.OperationStats, error)
	RotateStaffCalendarFeedToken(context.Context, int64, int64, string) (bool, domain.OperationStats, error)
}
