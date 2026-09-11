package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type caregiverBindingLocker struct {
	db *bun.DB
}

var caregiverCapabilityBindingTables = []string{
	"education.group_teacher",
	"education.group_substitution",
	"active.group_supervisors",
	"activities.supervisors",
}

func NewCaregiverBindingLocker(db *bun.DB) userModels.CaregiverBindingLocker {
	return &caregiverBindingLocker{db: db}
}

func (r *caregiverBindingLocker) LockCaregiverCapabilityBindings(ctx context.Context) error {
	db := base.GetDB(ctx, r.db)
	for _, tableName := range caregiverCapabilityBindingTables {
		if _, err := db.ExecContext(ctx, "LOCK TABLE "+tableName+" IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			return &modelBase.DatabaseError{
				Op:  "lock caregiver capability binding table " + tableName,
				Err: err,
			}
		}
	}
	return nil
}
