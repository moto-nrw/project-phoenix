package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	MinRFIDCardLength = 8
	MaxRFIDCardLength = 64
)

type RFIDCard struct {
	ID     string
	Active bool
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
