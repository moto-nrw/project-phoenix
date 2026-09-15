package auth

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// RFIDCardRepository implements auth.RFIDCardRepository. It lives with the
// identity-access adapters because users.rfid_cards is that owner's table
// (#2662); the People Directory only references a card through
// Person.TagID.
type RFIDCardRepository struct {
	*base.Repository[*auth.RFIDCard]
	db *bun.DB
}

// NewRFIDCardRepository creates a new RFIDCardRepository
func NewRFIDCardRepository(db *bun.DB) auth.RFIDCardRepository {
	repo := base.NewRepository[*auth.RFIDCard](db, "users.rfid_cards", "RfidCard")
	repo.TenantScoped = true
	return &RFIDCardRepository{
		Repository: repo,
		db:         db,
	}
}

// Delete overrides the base Delete method to match the interface
func (r *RFIDCardRepository) Delete(ctx context.Context, id string) error {
	// Normalize the tag ID to match stored format
	normalizedID := auth.NormalizeTagID(id)

	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.RFIDCard)(nil)).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`).
		Where(`"rfid_card".id = ?`, normalizedID)

	query = base.WithTenantFilter(ctx, query, "rfid_card")

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// FindByID overrides the base FindByID method to match the interface
func (r *RFIDCardRepository) FindByID(ctx context.Context, id string) (*auth.RFIDCard, error) {
	// Normalize the tag ID to match stored format
	normalizedID := auth.NormalizeTagID(id)

	card := new(auth.RFIDCard)
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
			Err: base.TranslateNotFound(err),
		}
	}

	return card, nil
}

// Deactivate sets an RFID card as inactive
func (r *RFIDCardRepository) Deactivate(ctx context.Context, id string) error {
	// Normalize the tag ID to match stored format
	normalizedID := auth.NormalizeTagID(id)

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.RFIDCard)(nil)).
		ModelTableExpr(`users.rfid_cards AS "rfid_card"`).
		Set("active = ?", false).
		Where(`"rfid_card".id = ?`, normalizedID)

	query = base.WithTenantFilter(ctx, query, "rfid_card")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "deactivate",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "deactivate rfid_card")
}

// Legacy method to maintain compatibility with old interface
func (r *RFIDCardRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.RFIDCard, error) {
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
