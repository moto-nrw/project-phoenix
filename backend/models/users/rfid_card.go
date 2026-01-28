package users

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// RFID card validation configuration
var (
	// MinRFIDCardLength is the minimum allowed length for RFID card IDs
	MinRFIDCardLength = 8

	// MaxRFIDCardLength is the maximum allowed length for RFID card IDs
	MaxRFIDCardLength = 64
)

const rfidCardTableName = "users.rfid_cards"

// RFIDCard represents a physical RFID card used for identification and access
type RFIDCard struct {
	base.StringIDModel `bun:"schema:users,table:rfid_cards"`
	Active             bool `bun:"active,notnull,default:true" json:"active"`
}

func (r *RFIDCard) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(rfidCardTableName)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(rfidCardTableName)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(rfidCardTableName)
	}
	return nil
}

// TableName returns the database table name
func (r *RFIDCard) TableName() string {
	return rfidCardTableName
}

// Validate ensures the RFID card data is valid
func (r *RFIDCard) Validate() error {
	if r.ID == "" {
		return errors.New("RFID card ID is required")
	}

	// Trim spaces from ID
	r.ID = strings.TrimSpace(r.ID)

	// Normalize RFID tag format - remove common separators
	r.ID = strings.ReplaceAll(r.ID, ":", "")
	r.ID = strings.ReplaceAll(r.ID, "-", "")
	r.ID = strings.ReplaceAll(r.ID, " ", "")

	// Convert to uppercase for consistency
	r.ID = strings.ToUpper(r.ID)

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
