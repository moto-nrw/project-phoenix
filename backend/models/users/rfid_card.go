package users

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// RFID card validation configuration
var (
	// MinRFIDCardLength is the minimum allowed length for RFID card IDs
	MinRFIDCardLength = 8

	// MaxRFIDCardLength is the maximum allowed length for RFID card IDs
	MaxRFIDCardLength = 64
)

// RFIDCard represents a physical RFID card used for identification and access
type RFIDCard struct {
	base.StringIDModel
	Active bool `bun:"active,notnull,default:true" json:"active"`
}

// TableName returns the database table name
func (r *RFIDCard) TableName() string {
	return "users.rfid_cards"
}

// Validate ensures the RFID card data is valid
func (r *RFIDCard) Validate() error {
	if r.ID == "" {
		return errors.New("RFID card ID is required")
	}

	// Trim spaces from ID
	r.ID = strings.TrimSpace(r.ID)

	// Validate ID length
	idLength := len(r.ID)
	if idLength < MinRFIDCardLength {
		return fmt.Errorf("RFID card ID too short: minimum length is %d characters", MinRFIDCardLength)
	}
	if idLength > MaxRFIDCardLength {
		return fmt.Errorf("RFID card ID too long: maximum length is %d characters", MaxRFIDCardLength)
	}

	// Validate ID format (typically hexadecimal)
	hexPattern := regexp.MustCompile(`^[A-Fa-f0-9]+$`)
	if !hexPattern.MatchString(r.ID) {
		return errors.New("invalid RFID card ID format, must be hexadecimal")
	}

	// Normalize to uppercase for consistency
	r.ID = strings.ToUpper(r.ID)

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

// GetID returns the ID of the RFID card
func (r *RFIDCard) GetID() interface{} {
	return r.ID
}

// GetCreatedAt returns the creation timestamp of the RFID card
func (r *RFIDCard) GetCreatedAt() time.Time {
	return r.CreatedAt
}

// GetUpdatedAt returns the last update timestamp of the RFID card
func (r *RFIDCard) GetUpdatedAt() time.Time {
	return r.UpdatedAt
}
