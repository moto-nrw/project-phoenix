// backend/database/repositories/users/rfid_card.go
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// RFIDCardRepository implements users.RFIDCardRepository interface
type RFIDCardRepository struct {
	*base.Repository[*users.RFIDCard]
	db *bun.DB
}

// NewRFIDCardRepository creates a new RFIDCardRepository
func NewRFIDCardRepository(db *bun.DB) users.RFIDCardRepository {
	return &RFIDCardRepository{
		Repository: base.NewRepository[*users.RFIDCard](db, "users.rfid_cards", "RFIDCard"),
		db:         db,
	}
}

// Delete overrides the base Delete method to match the interface
func (r *RFIDCardRepository) Delete(ctx context.Context, id string) error {
	// Normalize the tag ID to match stored format
	normalizedID := normalizeRFIDTagID(id)

	_, err := r.db.NewDelete().
		Model((*users.RFIDCard)(nil)).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`).
		Where(`"rfid_card".id = ?`, normalizedID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: err,
		}
	}

	return nil
}

// normalizeRFIDTagID normalizes RFID tag ID format (same logic as RFIDCard.Validate)
func normalizeRFIDTagID(tagID string) string {
	// Trim spaces
	tagID = strings.TrimSpace(tagID)

	// Remove common separators
	tagID = strings.ReplaceAll(tagID, ":", "")
	tagID = strings.ReplaceAll(tagID, "-", "")
	tagID = strings.ReplaceAll(tagID, " ", "")

	// Convert to uppercase
	return strings.ToUpper(tagID)
}

// FindByID overrides the base FindByID method to match the interface
func (r *RFIDCardRepository) FindByID(ctx context.Context, id string) (*users.RFIDCard, error) {
	// Normalize the tag ID to match stored format
	normalizedID := normalizeRFIDTagID(id)

	card := new(users.RFIDCard)
	err := r.db.NewSelect().
		Model(card).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`).
		Where(`"rfid_card".id = ?`, normalizedID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Card not found - return nil without error for auto-create logic
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return card, nil
}

// Activate sets an RFID card as active
func (r *RFIDCardRepository) Activate(ctx context.Context, id string) error {
	// Normalize the tag ID to match stored format
	normalizedID := normalizeRFIDTagID(id)

	_, err := r.db.NewUpdate().
		Model((*users.RFIDCard)(nil)).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`).
		Set("active = ?", true).
		Where(`"rfid_card".id = ?`, normalizedID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "activate",
			Err: err,
		}
	}

	return nil
}

// Deactivate sets an RFID card as inactive
func (r *RFIDCardRepository) Deactivate(ctx context.Context, id string) error {
	// Normalize the tag ID to match stored format
	normalizedID := normalizeRFIDTagID(id)

	_, err := r.db.NewUpdate().
		Model((*users.RFIDCard)(nil)).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`).
		Set("active = ?", false).
		Where(`"rfid_card".id = ?`, normalizedID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "deactivate",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *RFIDCardRepository) Create(ctx context.Context, card *users.RFIDCard) error {
	if card == nil {
		return fmt.Errorf("RFID card cannot be nil")
	}

	// Validate RFID card
	if err := card.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, card)
}

// Update overrides the base Update method to handle validation
func (r *RFIDCardRepository) Update(ctx context.Context, card *users.RFIDCard) error {
	if card == nil {
		return fmt.Errorf("RFID card cannot be nil")
	}

	// Validate RFID card
	if err := card.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, card)
}

// Legacy method to maintain compatibility with old interface
func (r *RFIDCardRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.RFIDCard, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "active":
				filter.Equal("active", value)
			default:
				// Default to exact match for other fields
				filter.Equal(field, value)
			}
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

// ListWithOptions provides a type-safe way to list RFID cards with query options
func (r *RFIDCardRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.RFIDCard, error) {
	var cards []*users.RFIDCard
	query := r.db.NewSelect().
		Model(&cards).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	return cards, nil
}

// FindCardWithPerson retrieves an RFID card with associated person data
func (r *RFIDCardRepository) FindCardWithPerson(ctx context.Context, id string) (*users.RFIDCard, error) {
	// Normalize the tag ID to match stored format
	normalizedID := normalizeRFIDTagID(id)

	// First get the card
	card, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then find the person associated with this card
	person := new(users.Person)
	err = r.db.NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".tag_id = ?`, normalizedID).
		Scan(ctx)

	// It's OK if we don't find a person (not an error)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, &modelBase.DatabaseError{
			Op:  "find person by tag ID",
			Err: err,
		}
	}

	// Return the card (with or without person)
	return card, nil
}
