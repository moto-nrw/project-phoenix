package database

import (
	"context"
	"database/sql"
	"errors"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"time"

	"github.com/uptrace/bun"
)

// SpecialistStore implements database operations for pedagogical specialist management
type SpecialistStore struct {
	db *bun.DB
}

// NewSpecialistStore returns a SpecialistStore
func NewSpecialistStore(db *bun.DB) *SpecialistStore {
	return &SpecialistStore{
		db: db,
	}
}

// CreateSpecialist creates a new pedagogical specialist
// If tagID is provided, a custom user with the tag ID is created or updated
// If accountID is provided, the user is linked to an existing account
func (s *SpecialistStore) CreateSpecialist(ctx context.Context, specialist *models2.PedagogicalSpecialist, tagID *string, accountID *int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// First check if we need to create a new user or use an existing one
	if specialist.UserID == 0 && specialist.CustomUser != nil {
		// Create a new user
		user := specialist.CustomUser

		// Set tag ID if provided
		if tagID != nil {
			user.TagID = tagID
		}

		// Set account ID if provided
		if accountID != nil {
			user.AccountID = accountID
		}

		// Set creation timestamp
		now := time.Now()
		user.CreatedAt = now
		user.ModifiedAt = now

		// Create the user
		_, err = tx.NewInsert().
			Model(user).
			Exec(ctx)

		if err != nil {
			return err
		}

		// Set the user ID on the specialist
		specialist.UserID = user.ID
	} else if specialist.UserID == 0 {
		return errors.New("either UserID or CustomUser must be provided")
	}

	// Create the specialist
	now := time.Now()
	specialist.CreatedAt = now
	specialist.ModifiedAt = now

	_, err = tx.NewInsert().
		Model(specialist).
		Exec(ctx)

	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetSpecialistByID retrieves a pedagogical specialist by ID with related data
func (s *SpecialistStore) GetSpecialistByID(ctx context.Context, id int64) (*models2.PedagogicalSpecialist, error) {
	specialist := new(models2.PedagogicalSpecialist)
	err := s.db.NewSelect().
		Model(specialist).
		Relation("CustomUser").
		Where("pedagogical_specialists.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	// Get assigned groups
	var groups []models2.Group
	err = s.db.NewSelect().
		Model(&groups).
		Join("JOIN group_supervisors gs ON gs.group_id = \"group\".id").
		Where("gs.specialist_id = ?", id).
		Relation("Room").
		Scan(ctx)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	specialist.Groups = groups

	return specialist, nil
}

// UpdateSpecialist updates an existing pedagogical specialist
func (s *SpecialistStore) UpdateSpecialist(ctx context.Context, specialist *models2.PedagogicalSpecialist) error {
	specialist.ModifiedAt = time.Now()

	_, err := s.db.NewUpdate().
		Model(specialist).
		Column("role", "is_password_otp", "modified_at").
		WherePK().
		Exec(ctx)

	return err
}

// DeleteSpecialist deletes a pedagogical specialist
func (s *SpecialistStore) DeleteSpecialist(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if this specialist is set as a representative for any groups
	isRepresentative, err := tx.NewSelect().
		Model((*models2.Group)(nil)).
		Where("representative_id = ?", id).
		Exists(ctx)

	if err != nil {
		return err
	}

	if isRepresentative {
		return errors.New("cannot delete specialist who is set as a representative for groups")
	}

	// Remove from all groups (delete junction table entries)
	_, err = tx.NewDelete().
		Model((*models2.GroupSupervisor)(nil)).
		Where("specialist_id = ?", id).
		Exec(ctx)

	if err != nil {
		return err
	}

	// Delete the specialist
	_, err = tx.NewDelete().
		Model((*models2.PedagogicalSpecialist)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return err
	}

	// Don't delete the associated user, as it may be referenced elsewhere

	return tx.Commit()
}

// ListSpecialists returns a list of all pedagogical specialists with optional filtering
func (s *SpecialistStore) ListSpecialists(ctx context.Context, filters map[string]interface{}) ([]models2.PedagogicalSpecialist, error) {
	var specialists []models2.PedagogicalSpecialist

	query := s.db.NewSelect().
		Model(&specialists).
		Relation("CustomUser")

	// Apply filters
	if role, ok := filters["role"].(string); ok && role != "" {
		query = query.Where("role = ?", role)
	}

	if searchTerm, ok := filters["search"].(string); ok && searchTerm != "" {
		query = query.Join("JOIN custom_users cu ON cu.id = pedagogical_specialists.user_id").
			Where("cu.first_name ILIKE ? OR cu.second_name ILIKE ?",
				"%"+searchTerm+"%", "%"+searchTerm+"%")
	}

	err := query.OrderExpr("pedagogical_specialists.id DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	// Load groups for each specialist
	for i := range specialists {
		var groups []models2.Group
		err = s.db.NewSelect().
			Model(&groups).
			Join("JOIN group_supervisors gs ON gs.group_id = \"group\".id").
			Where("gs.specialist_id = ?", specialists[i].ID).
			Relation("Room").
			Scan(ctx)

		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		specialists[i].Groups = groups
	}

	return specialists, nil
}

// ListSpecialistsWithoutSupervision returns specialists not assigned to any group
func (s *SpecialistStore) ListSpecialistsWithoutSupervision(ctx context.Context) ([]models2.PedagogicalSpecialist, error) {
	var specialists []models2.PedagogicalSpecialist

	// Get all specialists
	err := s.db.NewSelect().
		Model(&specialists).
		Relation("CustomUser").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	// Now filter out those who are already assigned to groups
	var result []models2.PedagogicalSpecialist
	for _, specialist := range specialists {
		// Check if this specialist has any group assignments
		exists, err := s.db.NewSelect().
			Model((*models2.GroupSupervisor)(nil)).
			Where("specialist_id = ?", specialist.ID).
			Exists(ctx)

		if err != nil {
			return nil, err
		}

		// If not assigned to any group, include in result
		if !exists {
			result = append(result, specialist)
		}
	}

	return result, nil
}

// AssignToGroup assigns a specialist to a group
func (s *SpecialistStore) AssignToGroup(ctx context.Context, specialistID, groupID int64) error {
	// Check if assignment already exists
	exists, err := s.db.NewSelect().
		Model((*models2.GroupSupervisor)(nil)).
		Where("specialist_id = ? AND group_id = ?", specialistID, groupID).
		Exists(ctx)

	if err != nil {
		return err
	}

	if exists {
		return errors.New("specialist is already assigned to this group")
	}

	// Create the assignment
	junction := &models2.GroupSupervisor{
		GroupID:      groupID,
		SpecialistID: specialistID,
		CreatedAt:    time.Now(),
	}

	_, err = s.db.NewInsert().
		Model(junction).
		Exec(ctx)

	return err
}

// RemoveFromGroup removes a specialist from a group
func (s *SpecialistStore) RemoveFromGroup(ctx context.Context, specialistID, groupID int64) error {
	// Delete the junction table entry
	_, err := s.db.NewDelete().
		Model((*models2.GroupSupervisor)(nil)).
		Where("specialist_id = ? AND group_id = ?", specialistID, groupID).
		Exec(ctx)

	return err
}

// ListAssignedGroups returns all groups a specialist is assigned to
func (s *SpecialistStore) ListAssignedGroups(ctx context.Context, specialistID int64) ([]models2.Group, error) {
	var groups []models2.Group

	err := s.db.NewSelect().
		Model(&groups).
		Join("JOIN group_supervisors gs ON gs.group_id = \"group\".id").
		Where("gs.specialist_id = ?", specialistID).
		Relation("Room").
		Relation("Representative").
		Relation("Representative.CustomUser").
		Scan(ctx)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return groups, nil
}
