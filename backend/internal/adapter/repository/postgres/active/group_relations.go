package active

import (
	"context"
	"database/sql"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// FindWithRelations retrieves a group with its associated relations
func (r *GroupRepository) FindWithRelations(ctx context.Context, id int64) (*active.Group, error) {
	group := new(active.Group)
	err := r.db.NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with relations",
			Err: err,
		}
	}

	return group, nil
}

// FindWithVisits retrieves a group with its associated visits
func (r *GroupRepository) FindWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	// First get the group
	group := new(active.Group)
	err := r.db.NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with visits - group",
			Err: err,
		}
	}

	// Then get the visits separately (Relation() doesn't work with multi-schema)
	var visits []*active.Visit
	err = r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".active_group_id = ?`, id).
		Scan(ctx)

	if err != nil && err != sql.ErrNoRows {
		return nil, &modelBase.DatabaseError{
			Op:  "find with visits - visits",
			Err: err,
		}
	}

	group.Visits = visits
	return group, nil
}

// FindWithSupervisors retrieves a group with its associated supervisors
func (r *GroupRepository) FindWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	// First get the group
	group := new(active.Group)
	err := r.db.NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with supervisors - group",
			Err: err,
		}
	}

	// Then get the supervisors
	var supervisors []*active.GroupSupervisor
	err = r.db.NewSelect().
		Model(&supervisors).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".group_id = ?`, id).
		Scan(ctx)

	if err != nil {
		// Don't fail if no supervisors found
		if err != sql.ErrNoRows {
			return nil, &modelBase.DatabaseError{
				Op:  "find with supervisors - supervisors",
				Err: err,
			}
		}
	}

	group.Supervisors = supervisors
	return group, nil
}
