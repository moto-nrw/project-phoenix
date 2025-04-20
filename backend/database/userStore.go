package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models"

	"github.com/uptrace/bun"
)

// UserStore implements database operations for user management
type UserStore struct {
	db *bun.DB
}

// NewUserStore returns a UserStore
func NewUserStore(db *bun.DB) *UserStore {
	return &UserStore{
		db: db,
	}
}

// GetCustomUserByID retrieves a CustomUser by ID
func (s *UserStore) GetCustomUserByID(ctx context.Context, id int64) (*models.CustomUser, error) {
	user := new(models.CustomUser)
	err := s.db.NewSelect().
		Model(user).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetCustomUserByTagID retrieves a CustomUser by RFID tag ID
func (s *UserStore) GetCustomUserByTagID(ctx context.Context, tagID string) (*models.CustomUser, error) {
	user := new(models.CustomUser)
	err := s.db.NewSelect().
		Model(user).
		Where("tag_id = ?", tagID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetCustomUserByAccountID retrieves a CustomUser by linked Account ID
func (s *UserStore) GetCustomUserByAccountID(ctx context.Context, accountID int64) (*models.CustomUser, error) {
	user := new(models.CustomUser)
	err := s.db.NewSelect().
		Model(user).
		Where("account_id = ?", accountID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return user, nil
}

// CreateCustomUser creates a new CustomUser
func (s *UserStore) CreateCustomUser(ctx context.Context, user *models.CustomUser) error {
	_, err := s.db.NewInsert().
		Model(user).
		Exec(ctx)

	return err
}

// UpdateCustomUser updates an existing CustomUser
func (s *UserStore) UpdateCustomUser(ctx context.Context, user *models.CustomUser) error {
	_, err := s.db.NewUpdate().
		Model(user).
		WherePK().
		Exec(ctx)

	return err
}

// DeleteCustomUser deletes a CustomUser
func (s *UserStore) DeleteCustomUser(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Model((*models.CustomUser)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

// UpdateTagID updates the RFID tag ID for a user
func (s *UserStore) UpdateTagID(ctx context.Context, userID int64, tagID string) error {
	_, err := s.db.NewUpdate().
		Model((*models.CustomUser)(nil)).
		Set("tag_id = ?", tagID).
		Where("id = ?", userID).
		Exec(ctx)

	return err
}

// ListCustomUsers returns a list of all CustomUsers
func (s *UserStore) ListCustomUsers(ctx context.Context) ([]models.CustomUser, error) {
	var users []models.CustomUser
	err := s.db.NewSelect().
		Model(&users).
		Order("first_name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return users, nil
}

// GetSpecialistByID retrieves a PedagogicalSpecialist by ID with related CustomUser
func (s *UserStore) GetSpecialistByID(ctx context.Context, id int64) (*models.PedagogicalSpecialist, error) {
	specialist := new(models.PedagogicalSpecialist)
	err := s.db.NewSelect().
		Model(specialist).
		Relation("CustomUser").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return specialist, nil
}

// GetSpecialistByUserID retrieves a PedagogicalSpecialist by UserID with related CustomUser
func (s *UserStore) GetSpecialistByUserID(ctx context.Context, userID int64) (*models.PedagogicalSpecialist, error) {
	specialist := new(models.PedagogicalSpecialist)
	err := s.db.NewSelect().
		Model(specialist).
		Relation("CustomUser").
		Where("user_id = ?", userID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return specialist, nil
}

// CreateSpecialist creates a new PedagogicalSpecialist
func (s *UserStore) CreateSpecialist(ctx context.Context, specialist *models.PedagogicalSpecialist) error {
	_, err := s.db.NewInsert().
		Model(specialist).
		Exec(ctx)

	return err
}

// UpdateSpecialist updates an existing PedagogicalSpecialist
func (s *UserStore) UpdateSpecialist(ctx context.Context, specialist *models.PedagogicalSpecialist) error {
	_, err := s.db.NewUpdate().
		Model(specialist).
		WherePK().
		Exec(ctx)

	return err
}

// DeleteSpecialist deletes a PedagogicalSpecialist
func (s *UserStore) DeleteSpecialist(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Model((*models.PedagogicalSpecialist)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

// ListSpecialists returns a list of all PedagogicalSpecialists with related CustomUser
func (s *UserStore) ListSpecialists(ctx context.Context) ([]models.PedagogicalSpecialist, error) {
	var specialists []models.PedagogicalSpecialist
	err := s.db.NewSelect().
		Model(&specialists).
		Relation("CustomUser").
		Order("custom_user.first_name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return specialists, nil
}

// ListSpecialistsWithoutSupervision returns a list of specialists with no supervision duties
func (s *UserStore) ListSpecialistsWithoutSupervision(ctx context.Context) ([]models.PedagogicalSpecialist, error) {
	var specialists []models.PedagogicalSpecialist

	// This would need to be adjusted once Group_Supervisor table is implemented
	// For now, return all specialists as a placeholder
	err := s.db.NewSelect().
		Model(&specialists).
		Relation("CustomUser").
		Order("custom_user.first_name ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return specialists, nil
}

// CreateUserFromAccount creates a CustomUser and links it to an existing Account
func (s *UserStore) CreateUserFromAccount(ctx context.Context, account *userpass.Account, firstName, secondName string) (*models.CustomUser, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	accountID := int64(account.ID)

	// Check if user already exists for this account
	exists, err := tx.NewSelect().
		Model((*models.CustomUser)(nil)).
		Where("account_id = ?", accountID).
		Exists(ctx)

	if err != nil {
		return nil, err
	}

	if exists {
		return nil, errors.New("a user already exists for this account")
	}

	// Create the custom user
	user := &models.CustomUser{
		FirstName:  firstName,
		SecondName: secondName,
		AccountID:  &accountID,
	}

	_, err = tx.NewInsert().
		Model(user).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return user, nil
}

// CreateDevice creates a new Device
func (s *UserStore) CreateDevice(ctx context.Context, device *models.Device) error {
	_, err := s.db.NewInsert().
		Model(device).
		Exec(ctx)

	return err
}

// GetDeviceByID retrieves a Device by ID
func (s *UserStore) GetDeviceByID(ctx context.Context, id int64) (*models.Device, error) {
	device := new(models.Device)
	err := s.db.NewSelect().
		Model(device).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return device, nil
}

// GetDeviceByDeviceID retrieves a Device by its unique device identifier
func (s *UserStore) GetDeviceByDeviceID(ctx context.Context, deviceID string) (*models.Device, error) {
	device := new(models.Device)
	err := s.db.NewSelect().
		Model(device).
		Where("device_id = ?", deviceID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return device, nil
}

// DeleteDevice deletes a Device
func (s *UserStore) DeleteDevice(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Model((*models.Device)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

// ListDevicesByUserID returns all devices for a user
func (s *UserStore) ListDevicesByUserID(ctx context.Context, userID int64) ([]models.Device, error) {
	var devices []models.Device
	err := s.db.NewSelect().
		Model(&devices).
		Where("user_id = ?", userID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return devices, nil
}
