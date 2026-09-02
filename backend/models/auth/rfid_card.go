package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// RFID card validation configuration
var (
	// MinRFIDCardLength is the minimum allowed length for RFID card IDs
	MinRFIDCardLength = 8

	// MaxRFIDCardLength is the maximum allowed length for RFID card IDs
	MaxRFIDCardLength = 64
)

// RFIDCard represents a physical RFID card used for identification and
// access. The table users.rfid_cards is owned by identity-access (#2662);
// the People Directory only references a card through Person.TagID.
type RFIDCard struct {
	base.StringIDModel `bun:"schema:users,table:rfid_cards"`
	base.TenantModel
	Active bool `bun:"active,notnull,default:true" json:"active"`
}

// NormalizeTagID normalizes an RFID tag ID: trims spaces, removes the
// common separators (: - space), and uppercases. Canonical implementation
// shared by RFIDCard.Validate, the repositories, and api/iot.
func NormalizeTagID(tagID string) string {
	tagID = strings.TrimSpace(tagID)
	tagID = strings.ReplaceAll(tagID, ":", "")
	tagID = strings.ReplaceAll(tagID, "-", "")
	tagID = strings.ReplaceAll(tagID, " ", "")
	return strings.ToUpper(tagID)
}

// Validate ensures the RFID card data is valid
func (r *RFIDCard) Validate() error {
	if r.ID == "" {
		return errors.New("RFID card ID is required")
	}

	// Normalize the RFID tag format for consistency
	r.ID = NormalizeTagID(r.ID)

	// Validate ID length after normalization
	idLength := len(r.ID)
	if idLength < MinRFIDCardLength {
		return fmt.Errorf("RFID card ID too short: minimum length is %d characters", MinRFIDCardLength)
	}
	if idLength > MaxRFIDCardLength {
		return fmt.Errorf("RFID card ID too long: maximum length is %d characters", MaxRFIDCardLength)
	}

	// Validate ID format (must be hexadecimal after normalization)
	hexPattern := regexp.MustCompile(`^[A-F0-9]+$`)
	if !hexPattern.MatchString(r.ID) {
		return errors.New("invalid RFID card ID format, must be hexadecimal")
	}

	return nil
}

// IsActive returns whether the RFID card is active
func (r *RFIDCard) IsActive() bool {
	return r.Active
}

// Activate sets the RFID card as active
func (r *RFIDCard) Activate() {
	r.Active = true
}

// Deactivate sets the RFID card as inactive
func (r *RFIDCard) Deactivate() {
	r.Active = false
}

// RFIDCardRepository defines operations for managing RFID cards
type RFIDCardRepository interface {
	// Create inserts a new RFID card into the database
	Create(ctx context.Context, card *RFIDCard) error

	// FindByID retrieves an RFID card by its ID
	FindByID(ctx context.Context, id string) (*RFIDCard, error)

	// Update updates an existing RFID card
	Update(ctx context.Context, card *RFIDCard) error

	// Delete removes an RFID card
	Delete(ctx context.Context, id string) error

	// List retrieves RFID cards matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*RFIDCard, error)

	// Deactivate sets an RFID card as inactive
	Deactivate(ctx context.Context, id string) error
}
