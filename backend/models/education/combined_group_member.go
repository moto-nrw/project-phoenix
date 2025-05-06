package education

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// CombinedGroupMember represents a member group in a combined group
type CombinedGroupMember struct {
	base.Model
	CombinedGroupID int64 `bun:"combined_group_id,notnull" json:"combined_group_id"`
	GroupID         int64 `bun:"group_id,notnull" json:"group_id"`

	// Relations
	CombinedGroup *CombinedGroup `bun:"rel:belongs-to,join:combined_group_id=id" json:"combined_group,omitempty"`
	Group         *Group         `bun:"rel:belongs-to,join:group_id=id" json:"group,omitempty"`
}

// TableName returns the table name for the CombinedGroupMember model
func (cgm *CombinedGroupMember) TableName() string {
	return "education.combined_group_members"
}

// GetID returns the combined group member ID
func (cgm *CombinedGroupMember) GetID() interface{} {
	return cgm.ID
}

// GetCreatedAt returns the creation timestamp
func (cgm *CombinedGroupMember) GetCreatedAt() time.Time {
	return cgm.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (cgm *CombinedGroupMember) GetUpdatedAt() time.Time {
	return cgm.CreatedAt // This model only has created_at, no updated_at
}

// Validate validates the combined group member fields
func (cgm *CombinedGroupMember) Validate() error {
	if cgm.CombinedGroupID <= 0 {
		return errors.New("combined group ID is required")
	}

	if cgm.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	return nil
}

// CombinedGroupMemberRepository defines operations for working with combined group members
type CombinedGroupMemberRepository interface {
	base.Repository[*CombinedGroupMember]
	FindByCombinedGroup(ctx context.Context, combinedGroupID int64) ([]*CombinedGroupMember, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*CombinedGroupMember, error)
	FindByCombinedGroupAndGroup(ctx context.Context, combinedGroupID, groupID int64) (*CombinedGroupMember, error)
	DeleteByCombinedGroup(ctx context.Context, combinedGroupID int64) error
	DeleteByGroup(ctx context.Context, groupID int64) error
}

// DefaultCombinedGroupMemberRepository is the default implementation of CombinedGroupMemberRepository
type DefaultCombinedGroupMemberRepository struct {
	db *bun.DB
}

// NewCombinedGroupMemberRepository creates a new combined group member repository
func NewCombinedGroupMemberRepository(db *bun.DB) CombinedGroupMemberRepository {
	return &DefaultCombinedGroupMemberRepository{db: db}
}

// Create inserts a new combined group member into the database
func (r *DefaultCombinedGroupMemberRepository) Create(ctx context.Context, combinedGroupMember *CombinedGroupMember) error {
	if err := combinedGroupMember.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(combinedGroupMember).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a combined group member by its ID
func (r *DefaultCombinedGroupMemberRepository) FindByID(ctx context.Context, id interface{}) (*CombinedGroupMember, error) {
	combinedGroupMember := new(CombinedGroupMember)
	err := r.db.NewSelect().Model(combinedGroupMember).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return combinedGroupMember, nil
}

// FindByCombinedGroup retrieves all members of a combined group
func (r *DefaultCombinedGroupMemberRepository) FindByCombinedGroup(ctx context.Context, combinedGroupID int64) ([]*CombinedGroupMember, error) {
	var combinedGroupMembers []*CombinedGroupMember
	err := r.db.NewSelect().
		Model(&combinedGroupMembers).
		Where("combined_group_id = ?", combinedGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_combined_group", Err: err}
	}
	return combinedGroupMembers, nil
}

// FindByGroup retrieves all combined group memberships for a group
func (r *DefaultCombinedGroupMemberRepository) FindByGroup(ctx context.Context, groupID int64) ([]*CombinedGroupMember, error) {
	var combinedGroupMembers []*CombinedGroupMember
	err := r.db.NewSelect().
		Model(&combinedGroupMembers).
		Where("group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return combinedGroupMembers, nil
}

// FindByCombinedGroupAndGroup retrieves a combined group member by combined group and group
func (r *DefaultCombinedGroupMemberRepository) FindByCombinedGroupAndGroup(ctx context.Context, combinedGroupID, groupID int64) (*CombinedGroupMember, error) {
	combinedGroupMember := new(CombinedGroupMember)
	err := r.db.NewSelect().
		Model(combinedGroupMember).
		Where("combined_group_id = ?", combinedGroupID).
		Where("group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_combined_group_and_group", Err: err}
	}
	return combinedGroupMember, nil
}

// DeleteByCombinedGroup deletes all members of a combined group
func (r *DefaultCombinedGroupMemberRepository) DeleteByCombinedGroup(ctx context.Context, combinedGroupID int64) error {
	_, err := r.db.NewDelete().
		Model((*CombinedGroupMember)(nil)).
		Where("combined_group_id = ?", combinedGroupID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_combined_group", Err: err}
	}
	return nil
}

// DeleteByGroup deletes all combined group memberships for a group
func (r *DefaultCombinedGroupMemberRepository) DeleteByGroup(ctx context.Context, groupID int64) error {
	_, err := r.db.NewDelete().
		Model((*CombinedGroupMember)(nil)).
		Where("group_id = ?", groupID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_group", Err: err}
	}
	return nil
}

// Update updates an existing combined group member
func (r *DefaultCombinedGroupMemberRepository) Update(ctx context.Context, combinedGroupMember *CombinedGroupMember) error {
	if err := combinedGroupMember.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(combinedGroupMember).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a combined group member
func (r *DefaultCombinedGroupMemberRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*CombinedGroupMember)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves combined group members matching the filters
func (r *DefaultCombinedGroupMemberRepository) List(ctx context.Context, filters map[string]interface{}) ([]*CombinedGroupMember, error) {
	var combinedGroupMembers []*CombinedGroupMember
	query := r.db.NewSelect().Model(&combinedGroupMembers)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return combinedGroupMembers, nil
}
