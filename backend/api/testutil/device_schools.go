package testutil

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	"github.com/uptrace/bun"
)

// DeviceSchools answers the device school guard straight from the seeded
// platform.schools rows, so handler tests exercise the deleted-school check
// against real fixtures.
func DeviceSchools(db *bun.DB) deviceauth.SchoolDirectory {
	return deviceSchools{db: db}
}

type deviceSchools struct{ db *bun.DB }

func (d deviceSchools) FindSchool(ctx context.Context, id int64) (*deviceauth.School, error) {
	var deletedAt *time.Time
	err := d.db.NewSelect().
		TableExpr(`platform.schools AS "school"`).
		Column("school.deleted_at").
		Where(`"school".id = ?`, id).
		Scan(ctx, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &deviceauth.School{Deleted: deletedAt != nil}, nil
}
