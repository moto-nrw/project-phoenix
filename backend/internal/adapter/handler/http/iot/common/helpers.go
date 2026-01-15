package common

import (
	"strings"
)

// Package common contains shared helper functions used across multiple IoT API domains.

// NormalizeTagID normalizes RFID tag ID format (same logic as in person repository)
// Removes common separators (: - space) and converts to uppercase
func NormalizeTagID(tagID string) string {
	// Trim spaces
	tagID = strings.TrimSpace(tagID)

	// Remove common separators
	tagID = strings.ReplaceAll(tagID, ":", "")
	tagID = strings.ReplaceAll(tagID, "-", "")
	tagID = strings.ReplaceAll(tagID, " ", "")

	// Convert to uppercase
	return strings.ToUpper(tagID)
}
