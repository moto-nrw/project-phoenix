package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
)

type Store interface {
	Create(context.Context, domain.Announcement) (domain.Announcement, domain.OperationStats, error)
	Get(context.Context, int64) (domain.Announcement, domain.OperationStats, error)
	GetForMutation(context.Context, int64) (domain.Announcement, domain.OperationStats, error)
	Update(context.Context, domain.Announcement) (domain.Announcement, domain.OperationStats, error)
	Delete(context.Context, int64) (domain.OperationStats, error)
	List(context.Context, bool) ([]domain.Announcement, domain.OperationStats, error)
	Publish(context.Context, int64) (domain.OperationStats, error)
	Unpublish(context.Context, int64) (domain.OperationStats, error)
	MarkSeen(context.Context, int64, int64) (domain.OperationStats, error)
	MarkDismissed(context.Context, int64, int64) (domain.OperationStats, error)
	ViewStats(context.Context, int64) (domain.AnnouncementStats, domain.OperationStats, error)
}

type Audience interface {
	Unread(context.Context, int64, []string, int64, int64) ([]domain.Announcement, domain.OperationStats, error)
	CountUnread(context.Context, int64, []string, int64, int64) (int, domain.OperationStats, error)
	TargetCount(context.Context, domain.Announcement) (int, domain.OperationStats, error)
	ViewDetails(context.Context, int64) ([]domain.AnnouncementViewDetail, domain.OperationStats, error)
}

type Targets interface {
	CountOrganizationsByID(context.Context, []int64) (int, domain.OperationStats, error)
	CountSchoolsByID(context.Context, []int64) (int, domain.OperationStats, error)
}

type ViewerNames interface {
	NamesByAccount(context.Context, []int64) (map[int64]string, domain.OperationStats, error)
}

type Audit interface {
	Append(context.Context, domain.AuditEntry) (domain.OperationStats, error)
}

type Transaction interface {
	RunAdmin(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
