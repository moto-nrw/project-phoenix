package users

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// RFIDCard represents an RFID card in the system
type RFIDCard struct {
	base.StringIDModel
	Active bool `bun:"active,notnull,default:true" json:"active"`

	// Relations
	Persons []*Person `bun:"rel:has-many,join:id=tag_id" json:"persons,omitempty"`
}

// TableName returns the table name for the RFIDCard model
func (r *RFIDCard) TableName() string {
	return "users.rfid_cards"
}

// GetID returns the RFID card ID
func (r *RFIDCard) GetID() interface{} {
	return r.ID
}

// GetCreatedAt returns the creation timestamp
func (r *RFIDCard) GetCreatedAt() time.Time {
	return r.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (r *RFIDCard) GetUpdatedAt() time.Time {
	return r.UpdatedAt
}

// Validate validates the RFID card fields
func (r *RFIDCard) Validate() error {
	if r.ID == "" {
		return errors.New("RFID card ID is required")
	}
	return nil
}

// RFIDCardRepository defines operations for working with RFID cards
type RFIDCardRepository interface {
	base.Repository[*RFIDCard]
	FindActive(ctx context.Context) ([]*RFIDCard, error)
	Deactivate(ctx context.Context, id string) error
	Activate(ctx context.Context, id string) error
}

// DefaultRFIDCardRepository is the default implementation of RFIDCardRepository
type DefaultRFIDCardRepository struct {
	db *bun.DB
}

// NewRFIDCardRepository creates a new RFID card repository
func NewRFIDCardRepository(db *bun.DB) RFIDCardRepository {
	return &DefaultRFIDCardRepository{db: db}
}

// Create inserts a new RFID card into the database
func (r *DefaultRFIDCardRepository) Create(ctx context.Context, card *RFIDCard) error {
	if err := card.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(card).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves an RFID card by its ID
func (r *DefaultRFIDCardRepository) FindByID(ctx context.Context, id interface{}) (*RFIDCard, error) {
	card := new(RFIDCard)
	err := r.db.NewSelect().Model(card).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return card, nil
}

// Update updates an existing RFID card
func (r *DefaultRFIDCardRepository) Update(ctx context.Context, card *RFIDCard) error {
	if err := card.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(card).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes an RFID card
func (r *DefaultRFIDCardRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*RFIDCard)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves RFID cards matching the filters
func (r *DefaultRFIDCardRepository) List(ctx context.Context, filters map[string]interface{}) ([]*RFIDCard, error) {
	var cards []*RFIDCard
	query := r.db.NewSelect().Model(&cards)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return cards, nil
}

// FindActive retrieves all active RFID cards
func (r *DefaultRFIDCardRepository) FindActive(ctx context.Context) ([]*RFIDCard, error) {
	var cards []*RFIDCard
	err := r.db.NewSelect().
		Model(&cards).
		Where("active = ?", true).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return cards, nil
}

// Deactivate sets an RFID card as inactive
func (r *DefaultRFIDCardRepository) Deactivate(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().
		Model((*RFIDCard)(nil)).
		Set("active = ?", false).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "deactivate", Err: err}
	}
	return nil
}

// Activate sets an RFID card as active
func (r *DefaultRFIDCardRepository) Activate(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().
		Model((*RFIDCard)(nil)).
		Set("active = ?", true).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "activate", Err: err}
	}
	return nil
}
