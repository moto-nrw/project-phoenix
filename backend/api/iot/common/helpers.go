// Package common contains shared helper functions used across multiple IoT API domains.
package common

import (
	"github.com/moto-nrw/project-phoenix/models/users"
)

// NormalizeTagID normalizes RFID tag ID format. Canonical implementation:
// users.NormalizeTagID (kept next to the RFIDCard model).
func NormalizeTagID(tagID string) string {
	return users.NormalizeTagID(tagID)
}
