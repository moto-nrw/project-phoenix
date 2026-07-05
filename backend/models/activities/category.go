package activities

import (
	"errors"
	"regexp"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableActivitiesCategories is the schema-qualified table name for categories
const tableActivitiesCategories = "activities.categories"

// DefaultCategoryColor is the fallback display color used when a category has
// no color set.
const DefaultCategoryColor = "#CCCCCC"

// Category represents a category for activities
type Category struct {
	base.Model `bun:"schema:activities,table:categories"`
	base.TenantModel
	Name        string `bun:"name,notnull" json:"name"`
	Description string `bun:"description" json:"description,omitempty"`
	Color       string `bun:"color" json:"color,omitempty"`
}

func (c *Category) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActivitiesCategories)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActivitiesCategories)
	}
	return nil
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
