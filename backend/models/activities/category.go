package activities

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// DefaultCategoryColor is the fallback display color used when a category has
// no color set.
const DefaultCategoryColor = "#CCCCCC"

// ErrUnknownCategoryIDs is returned when a Kategorie↔Schichtart mapping write
// references category IDs that do not all exist in the current tenant. It must
// abort the write rather than silently clearing existing mappings (#1837
// follow-up). Callers map it to a 400.
var ErrUnknownCategoryIDs = errors.New("one or more category IDs do not exist in this tenant")

// Category represents a category for activities
type Category struct {
	base.Model `bun:"schema:activities,table:categories"`
	base.TenantModel
	Name        string `bun:"name,notnull" json:"name"`
	Description string `bun:"description" json:"description,omitempty"`
	Color       string `bun:"color" json:"color,omitempty"`
	IsSystem    bool   `bun:"is_system,notnull,default:false" json:"is_system"`
	// ShiftTypeID optionally maps this Timetable-Kategorie to a Dienstplan
	// Schichtart (schedule.shift_types, #1836). NULL = no mapping. Managed from
	// the "Schichtarten verwalten" modal; see #1837 follow-up.
	ShiftTypeID *int64 `bun:"shift_type_id" json:"shift_type_id,omitempty"`
	// ArchivedAt marks a category as retired (#2131). Archived categories stay
	// in the table so existing Termine and Aktivitäten keep resolving their
	// category, but they are no longer offered for new assignments. NULL =
	// active. Only active rows take part in the case-insensitive tenant/name
	// unique index.
	ArchivedAt *time.Time `bun:"archived_at" json:"archived_at,omitempty"`
}

// IsArchived reports whether the category has been retired and should no
// longer be offered when picking a category.
func (c *Category) IsArchived() bool {
	return c.ArchivedAt != nil
}

// Validate ensures category data is valid
func (c *Category) Validate() error {
	if c.Name == "" {
		return errors.New("category name is required")
	}

	// Trim spaces from name
	c.Name = strings.TrimSpace(c.Name)

	// Validate color if provided
	if c.Color != "" {
		// Add # prefix if missing
		if !strings.HasPrefix(c.Color, "#") {
			c.Color = "#" + c.Color
		}

		// Validate hex color format (#RRGGBB or #RGB)
		hexColorPattern := regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)
		if !hexColorPattern.MatchString(c.Color) {
			return errors.New("invalid color format, must be a valid hex color")
		}
	}

	return nil
}

// GetColorOrDefault returns the category color or a default color if not set
func (c *Category) GetColorOrDefault() string {
	if c.Color == "" {
		return DefaultCategoryColor // Default gray color
	}
	return c.Color
}

// HasDescription checks if the category has a description
func (c *Category) HasDescription() bool {
	return c.Description != ""
}
