package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type ParentCalendarFeedStore interface {
	FindParentCalendarFeedAccount(context.Context, int64) (domain.ParentCalendarFeedAccount, bool, domain.OperationStats, error)
	FindParentCalendarFeedOwner(context.Context, string) (domain.ParentCalendarFeedAccount, bool, domain.OperationStats, error)
	EnsureParentCalendarFeedToken(context.Context, int64, string) (string, domain.OperationStats, error)
	RotateParentCalendarFeedToken(context.Context, int64, string) (domain.OperationStats, error)
}
