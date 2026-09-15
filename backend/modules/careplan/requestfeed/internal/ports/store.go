package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/domain"
)

type Store interface {
	Active(context.Context, int64, int64) (bool, error)
	Create(context.Context, int64, int64, string) (bool, error)
	Rotate(context.Context, int64, int64, string) (bool, error)
	Resolve(context.Context, string) (domain.Subscription, bool, error)
	List(context.Context, int64, time.Time, domain.Access) ([]domain.Item, error)
}

type AccessResolver interface {
	Resolve(context.Context, int64, int64) (domain.Access, error)
}

type TokenCodec interface {
	New() (raw string, hash string, err error)
	Hash(raw string) string
}
