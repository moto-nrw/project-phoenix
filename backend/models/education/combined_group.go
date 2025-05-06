package education

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// CombinedGroup represents a combined education group
type CombinedGroup struct {
	base.Model
	Name         string     `bun:"name,notnull,unique" json:"name"`
	IsActive     bool       `bun:"is_active,notnull,default:true" json:"is_active"`
	ValidUntil   *time.Time `bun:"valid_until" json:"valid_until,omitempty"`
	AccessPolicy string     `bun:"access_policy,notnull,default:'all'" json:"access_policy"`

	// Relations
	Members  []*CombinedGroupMember  `bun:"rel:has-many,join:id=combined_group_id" json:"members,omitempty"`
	Teachers []*CombinedGroupTeacher `bun:"rel:has-many,join:id=combined_group_id" json:"teachers,omitempty"`
}

// TableName returns the table name for the CombinedGroup model
func (cg *CombinedGroup) TableName() string {
	return "education.combined_groups"
}

// GetID returns the combined group ID
func (cg *CombinedGroup) GetID() interface{} {
	return cg.ID
}

// GetCreatedAt returns the creation timestamp
func (cg *CombinedGroup) GetCreatedAt() time.Time {
	return cg.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (cg *CombinedGroup) GetUpdatedAt() time.Time {
	return cg.UpdatedAt
}

// Validate validates the combined group fields
func (cg *CombinedGroup) Validate() error {
	if strings.TrimSpace(cg.Name) == "" {
		return errors.New("combined group name is required")
	}

	if strings.TrimSpace(cg.AccessPolicy) == "" {
		return errors.New("access policy is required")
	}

	if cg.ValidUntil != nil && !cg.ValidUntil.IsZero() && cg.ValidUntil.Before(time.Now()) {
		return errors.New("valid until date must be in the future")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (cg *CombinedGroup) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := cg.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	cg.Name = strings.TrimSpace(cg.Name)
	cg.AccessPolicy = strings.TrimSpace(cg.AccessPolicy)

	// Set default access policy if empty
	if cg.AccessPolicy == "" {
		cg.AccessPolicy = "all"
	}

	return nil
}

// IsValid checks if the combined group is valid (active and not expired)
func (cg *CombinedGroup) IsValid() bool {
	return cg.IsActive && (cg.ValidUntil == nil || cg.ValidUntil.IsZero() || cg.ValidUntil.After(time.Now()))
}

// CombinedGroupRepository defines operations for working with combined groups
type CombinedGroupRepository interface {
	base.Repository[*CombinedGroup]
	FindByName(ctx context.Context, name string) (*CombinedGroup, error)
	FindActive(ctx context.Context) ([]*CombinedGroup, error)
	FindByAccessPolicy(ctx context.Context, accessPolicy string) ([]*CombinedGroup, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*CombinedGroup, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*CombinedGroup, error)
	UpdateStatus(ctx context.Context, id int64, isActive bool) error
}

// DefaultCombinedGroupRepository is the default implementation of CombinedGroupRepository
type DefaultCombinedGroupRepository struct {
	db *bun.DB
}

// NewCombinedGroupRepository creates a new combined group repository
func NewCombinedGroupRepository(db *bun.DB) CombinedGroupRepository {
	return &DefaultCombinedGroupRepository{db: db}
}

// Create inserts a new combined group into the database
func (r *DefaultCombinedGroupRepository) Create(ctx context.Context, combinedGroup *CombinedGroup) error {
	if err := combinedGroup.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(combinedGroup).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a combined group by its ID
func (r *DefaultCombinedGroupRepository) FindByID(ctx context.Context, id interface{}) (*CombinedGroup, error) {
	combinedGroup := new(CombinedGroup)
	err := r.db.NewSelect().Model(combinedGroup).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return combinedGroup, nil
}

// FindByName retrieves a combined group by its name
func (r *DefaultCombinedGroupRepository) FindByName(ctx context.Context, name string) (*CombinedGroup, error) {
	combinedGroup := new(CombinedGroup)
	err := r.db.NewSelect().Model(combinedGroup).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return combinedGroup, nil
}

// FindActive retrieves all active combined groups
func (r *DefaultCombinedGroupRepository) FindActive(ctx context.Context) ([]*CombinedGroup, error) {
	var combinedGroups []*CombinedGroup
	err := r.db.NewSelect().
		Model(&combinedGroups).
		Where("is_active = ?", true).
		Where("valid_until IS NULL OR valid_until > ?", time.Now()).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return combinedGroups, nil
}

// FindByAccessPolicy retrieves all combined groups with a specific access policy
func (r *DefaultCombinedGroupRepository) FindByAccessPolicy(ctx context.Context, accessPolicy string) ([]*CombinedGroup, error) {
	var combinedGroups []*CombinedGroup
	err := r.db.NewSelect().
		Model(&combinedGroups).
		Where("access_policy = ?", accessPolicy).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_access_policy", Err: err}
	}
	return combinedGroups, nil
}

// FindByGroup retrieves all combined groups that include a specific group
func (r *DefaultCombinedGroupRepository) FindByGroup(ctx context.Context, groupID int64) ([]*CombinedGroup, error) {
	var combinedGroups []*CombinedGroup
	err := r.db.NewSelect().
		Model(&combinedGroups).
		Join("JOIN education.combined_group_members cgm ON combined_groups.id = cgm.combined_group_id").
		Where("cgm.group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return combinedGroups, nil
}

// FindByTeacher retrieves all combined groups associated with a specific teacher
func (r *DefaultCombinedGroupRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*CombinedGroup, error) {
	var combinedGroups []*CombinedGroup
	err := r.db.NewSelect().
		Model(&combinedGroups).
		Join("JOIN education.combined_group_teacher cgt ON combined_groups.id = cgt.combined_group_id").
		Where("cgt.teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_teacher", Err: err}
	}
	return combinedGroups, nil
}

// UpdateStatus updates the active status of a combined group
func (r *DefaultCombinedGroupRepository) UpdateStatus(ctx context.Context, id int64, isActive bool) error {
	_, err := r.db.NewUpdate().
		Model((*CombinedGroup)(nil)).
		Set("is_active = ?", isActive).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_status", Err: err}
	}
	return nil
}

// Update updates an existing combined group
func (r *DefaultCombinedGroupRepository) Update(ctx context.Context, combinedGroup *CombinedGroup) error {
	if err := combinedGroup.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(combinedGroup).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a combined group
func (r *DefaultCombinedGroupRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*CombinedGroup)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves combined groups matching the filters
func (r *DefaultCombinedGroupRepository) List(ctx context.Context, filters map[string]interface{}) ([]*CombinedGroup, error) {
	var combinedGroups []*CombinedGroup
	query := r.db.NewSelect().Model(&combinedGroups)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return combinedGroups, nil
}
