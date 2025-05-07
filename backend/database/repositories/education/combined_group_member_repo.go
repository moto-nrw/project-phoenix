package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// CombinedGroupMemberRepository implements education.CombinedGroupMemberRepository
type CombinedGroupMemberRepository struct {
	db *bun.DB
}

// NewCombinedGroupMemberRepository creates a new combined group member repository
func NewCombinedGroupMemberRepository(db *bun.DB) education.CombinedGroupMemberRepository {
	return &CombinedGroupMemberRepository{db: db}
}

// Create inserts a new combined group member into the database
func (r *CombinedGroupMemberRepository) Create(ctx context.Context, cgm *education.CombinedGroupMember) error {
	if err := cgm.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(cgm).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a combined group member by its ID
func (r *CombinedGroupMemberRepository) FindByID(ctx context.Context, id interface{}) (*education.CombinedGroupMember, error) {
	cgm := new(education.CombinedGroupMember)
	err := r.db.NewSelect().Model(cgm).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return cgm, nil
}

// FindByCombinedGroup retrieves all members of a combined group
func (r *CombinedGroupMemberRepository) FindByCombinedGroup(ctx context.Context, combinedGroupID int64) ([]*education.CombinedGroupMember, error) {
	var members []*education.CombinedGroupMember
	err := r.db.NewSelect().
		Model(&members).
		Where("combined_group_id = ?", combinedGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_combined_group", Err: err}
	}
	return members, nil
}

// FindByGroup retrieves all combined group memberships for a group
func (r *CombinedGroupMemberRepository) FindByGroup(ctx context.Context, groupID int64) ([]*education.CombinedGroupMember, error) {
	var members []*education.CombinedGroupMember
	err := r.db.NewSelect().
		Model(&members).
		Where("group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return members, nil
}

// FindByCombinedGroupAndGroup retrieves a combined group member by combined group and group
func (r *CombinedGroupMemberRepository) FindByCombinedGroupAndGroup(ctx context.Context, combinedGroupID, groupID int64) (*education.CombinedGroupMember, error) {
	member := new(education.CombinedGroupMember)
	err := r.db.NewSelect().
		Model(member).
		Where("combined_group_id = ?", combinedGroupID).
		Where("group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_combined_group_and_group", Err: err}
	}
	return member, nil
}

// DeleteByCombinedGroup deletes all members of a combined group
func (r *CombinedGroupMemberRepository) DeleteByCombinedGroup(ctx context.Context, combinedGroupID int64) error {
	_, err := r.db.NewDelete().
		Model((*education.CombinedGroupMember)(nil)).
		Where("combined_group_id = ?", combinedGroupID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_combined_group", Err: err}
	}
	return nil
}

// DeleteByGroup deletes all combined group memberships for a group
func (r *CombinedGroupMemberRepository) DeleteByGroup(ctx context.Context, groupID int64) error {
	_, err := r.db.NewDelete().
		Model((*education.CombinedGroupMember)(nil)).
		Where("group_id = ?", groupID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_group", Err: err}
	}
	return nil
}

// Update updates an existing combined group member
func (r *CombinedGroupMemberRepository) Update(ctx context.Context, cgm *education.CombinedGroupMember) error {
	if err := cgm.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(cgm).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a combined group member
func (r *CombinedGroupMemberRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*education.CombinedGroupMember)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves combined group members matching the filters
func (r *CombinedGroupMemberRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.CombinedGroupMember, error) {
	var members []*education.CombinedGroupMember
	query := r.db.NewSelect().Model(&members)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return members, nil
}
