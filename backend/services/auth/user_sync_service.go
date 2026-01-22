// Package auth provides authentication and authorization services.
package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/logging"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// UserSyncParams contains parameters for syncing a BetterAuth user to Go backend.
type UserSyncParams struct {
	// BetterAuthUserID is the UUID from BetterAuth's user table
	BetterAuthUserID string
	// Email is the user's email address
	Email string
	// Name is the user's display name (will be split into first/last)
	Name string
	// OrganizationID is the BetterAuth organization UUID
	OrganizationID string
	// Role is the member's role in the organization (admin, member, owner)
	Role string
}

// UserSyncResult contains the result of syncing a user.
type UserSyncResult struct {
	PersonID  int64
	StaffID   int64
	TeacherID int64
}

// UserSyncService handles syncing BetterAuth users to Go backend.
type UserSyncService interface {
	// SyncUser creates Person, Staff, and optionally Teacher records for a BetterAuth user.
	// This is called by the internal API when BetterAuth's afterAcceptInvitation hook fires.
	SyncUser(ctx context.Context, params UserSyncParams) (*UserSyncResult, error)
}

// userSyncService implements UserSyncService.
type userSyncService struct {
	db          *bun.DB
	personRepo  userModels.PersonRepository
	staffRepo   userModels.StaffRepository
	teacherRepo userModels.TeacherRepository
}

// NewUserSyncService creates a new UserSyncService.
func NewUserSyncService(
	db *bun.DB,
	personRepo userModels.PersonRepository,
	staffRepo userModels.StaffRepository,
	teacherRepo userModels.TeacherRepository,
) UserSyncService {
	return &userSyncService{
		db:          db,
		personRepo:  personRepo,
		staffRepo:   staffRepo,
		teacherRepo: teacherRepo,
	}
}

// SyncUser creates Person, Staff, and optionally Teacher records for a BetterAuth user.
func (s *userSyncService) SyncUser(ctx context.Context, params UserSyncParams) (*UserSyncResult, error) {
	// Validate required fields
	if params.BetterAuthUserID == "" {
		return nil, fmt.Errorf("betterauth_user_id is required")
	}
	if params.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if params.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if params.OrganizationID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}

	// Parse name into first and last
	firstName, lastName := parseName(params.Name)

	var result UserSyncResult

	// Use a transaction to ensure atomicity and set RLS context
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Set RLS context using the organization ID
		// This allows the insert to pass the RLS trigger check
		_, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.ogs_id = '%s'", params.OrganizationID))
		if err != nil {
			return fmt.Errorf("failed to set RLS context: %w", err)
		}

		// Create person
		// Note: BUN's BeforeAppendModel hooks don't apply to INSERT queries,
		// so we must explicitly specify the table with ModelTableExpr
		person := &userModels.Person{
			FirstName: firstName,
			LastName:  lastName,
		}
		if _, err := tx.NewInsert().Model(person).ModelTableExpr("users.persons").Exec(ctx); err != nil {
			return fmt.Errorf("failed to create person: %w", err)
		}
		result.PersonID = person.ID

		// Create staff (all organization members are staff)
		staff := &userModels.Staff{
			PersonID: person.ID,
		}
		if _, err := tx.NewInsert().Model(staff).ModelTableExpr("users.staff").Exec(ctx); err != nil {
			return fmt.Errorf("failed to create staff: %w", err)
		}
		result.StaffID = staff.ID

		// Create teacher for admin/owner roles
		if isAdminRole(params.Role) {
			teacher := &userModels.Teacher{
				StaffID: staff.ID,
				Role:    mapRoleToTeacherRole(params.Role),
			}
			if _, err := tx.NewInsert().Model(teacher).ModelTableExpr("users.teachers").Exec(ctx); err != nil {
				// Log but don't fail - staff was created successfully
				if logging.Logger != nil {
					logging.Logger.WithError(err).WithFields(map[string]interface{}{
						"betterauth_user_id": params.BetterAuthUserID,
						"staff_id":           staff.ID,
						"role":               params.Role,
					}).Warn("UserSyncService: failed to create teacher record")
				}
			} else {
				result.TeacherID = teacher.ID
			}
		}

		return nil
	})

	if err != nil {
		if logging.Logger != nil {
			logging.Logger.WithError(err).WithFields(map[string]interface{}{
				"betterauth_user_id": params.BetterAuthUserID,
				"email":              params.Email,
				"organization_id":    params.OrganizationID,
			}).Error("UserSyncService: failed to sync user")
		}
		return nil, err
	}

	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"betterauth_user_id": params.BetterAuthUserID,
			"email":              params.Email,
			"organization_id":    params.OrganizationID,
			"person_id":          result.PersonID,
			"staff_id":           result.StaffID,
			"teacher_id":         result.TeacherID,
			"role":               params.Role,
		}).Info("UserSyncService: user synced successfully")
	}

	return &result, nil
}

// parseName splits a display name into first and last name.
func parseName(name string) (firstName, lastName string) {
	name = strings.TrimSpace(name)
	parts := strings.SplitN(name, " ", 2)

	if len(parts) == 1 {
		return parts[0], ""
	}

	return parts[0], strings.TrimSpace(parts[1])
}

// isAdminRole checks if the role should have teacher privileges.
func isAdminRole(role string) bool {
	role = strings.ToLower(role)
	return role == "admin" || role == "owner" || role == "ogsadmin" || role == "traegeradmin" || role == "bueroadmin"
}

// mapRoleToTeacherRole maps BetterAuth roles to Go teacher roles.
func mapRoleToTeacherRole(role string) string {
	role = strings.ToLower(role)
	switch role {
	case "owner":
		return "Leitung"
	case "admin", "ogsadmin":
		return "Gruppenleitung"
	case "traegeradmin":
		return "Träger-Admin"
	case "bueroadmin":
		return "Büro-Admin"
	default:
		return "Mitarbeiter"
	}
}
