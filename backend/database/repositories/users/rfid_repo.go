package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// RFIDCardRepository implements users.RFIDCardRepository
type RFIDCardRepository struct {
	db *bun.DB
}

// NewRFIDCardRepository creates a new RFID card repository
func NewRFIDCardRepository(db *bun.DB) users.RFIDCardRepository {
	return &RFIDCardRepository{db: db}
}

// Create inserts a new RFID card into the database
func (r *RFIDCardRepository) Create(ctx context.Context, card *users.RFIDCard) error {
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
func (r *RFIDCardRepository) FindByID(ctx context.Context, id interface{}) (*users.RFIDCard, error) {
	card := new(users.RFIDCard)
	err := r.db.NewSelect().Model(card).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return card, nil
}

// Update updates an existing RFID card
func (r *RFIDCardRepository) Update(ctx context.Context, card *users.RFIDCard) error {
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
func (r *RFIDCardRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*users.RFIDCard)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves RFID cards matching the filters
func (r *RFIDCardRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.RFIDCard, error) {
	var cards []*users.RFIDCard
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
func (r *RFIDCardRepository) FindActive(ctx context.Context) ([]*users.RFIDCard, error) {
	var cards []*users.RFIDCard
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
func (r *RFIDCardRepository) Deactivate(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().
		Model((*users.RFIDCard)(nil)).
		Set("active = ?", false).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "deactivate", Err: err}
	}
	return nil
}

// Activate sets an RFID card as active
func (r *RFIDCardRepository) Activate(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().
		Model((*users.RFIDCard)(nil)).
		Set("active = ?", true).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "activate", Err: err}
	}
	return nil
}
