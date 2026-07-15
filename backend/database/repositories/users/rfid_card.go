// backend/database/repositories/users/rfid_card.go
package users

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
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
	repo := base.NewRepository[*users.RFIDCard](db, "users.rfid_cards", "RfidCard")
	repo.TenantScoped = true
	return &RFIDCardRepository{
		Repository: repo,
		db:         db,
	}
}

// Delete overrides the base Delete method to match the interface
func (r *RFIDCardRepository) Delete(ctx context.Context, id string) error {
	// Normalize the tag ID to match stored format
	normalizedID := users.NormalizeTagID(id)

	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*users.RFIDCard)(nil)).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`).
		Where(`"rfid_card".id = ?`, normalizedID)

	query = base.WithTenantFilter(ctx, query, "rfid_card")

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: err,
		}
	}

	return nil
}

// FindByID overrides the base FindByID method to match the interface
func (r *RFIDCardRepository) FindByID(ctx context.Context, id string) (*users.RFIDCard, error) {
	// Normalize the tag ID to match stored format
	normalizedID := users.NormalizeTagID(id)

	card := new(users.RFIDCard)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(card).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`).
		Where(`"rfid_card".id = ?`, normalizedID)

	query = base.WithTenantFilter(ctx, query, "rfid_card")

	err := query.Scan(ctx)

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

// Deactivate sets an RFID card as inactive
func (r *RFIDCardRepository) Deactivate(ctx context.Context, id string) error {
	// Normalize the tag ID to match stored format
	normalizedID := users.NormalizeTagID(id)

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.RFIDCard)(nil)).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`).
		Set("active = ?", false).
		Where(`"rfid_card".id = ?`, normalizedID)

	query = base.WithTenantFilter(ctx, query, "rfid_card")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "deactivate",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "deactivate rfid_card")
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

// FindCardWithPerson retrieves an RFID card with associated person data
func (r *RFIDCardRepository) FindCardWithPerson(ctx context.Context, id string) (*users.RFIDCard, error) {
	// Normalize the tag ID to match stored format
	normalizedID := users.NormalizeTagID(id)

	// First get the card
	card, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then find the person associated with this card
	person := new(users.Person)
	personQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".tag_id = ?`, normalizedID)

	personQuery = base.WithTenantFilter(ctx, personQuery, "person")

	err = personQuery.Scan(ctx)

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
