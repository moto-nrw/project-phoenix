package users

import (
	"errors"
	"regexp"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
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

	// Validate ID format (typically hexadecimal)
	hexPattern := regexp.MustCompile(`^[A-Fa-f0-9]+$`)
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