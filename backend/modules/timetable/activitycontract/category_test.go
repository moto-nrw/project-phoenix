package activitycontract

import (
	"testing"
)

func TestCategoryValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category *Category
		wantErr  bool
	}{
		{
			name: "Valid category with color",
			category: &Category{
				Name:        "Test Category",
				Description: "A test category",
				Color:       "#FF5733",
			},
			wantErr: false,
		},
		{
			name: "Valid category without color",
			category: &Category{
				Name:        "Test Category",
				Description: "A test category",
			},
			wantErr: false,
		},
		{
			name: "Valid category with color without hash",
			category: &Category{
				Name:        "Test Category",
				Description: "A test category",
				Color:       "FF5733",
			},
			wantErr: false,
		},
		{
			name: "Missing name",
			category: &Category{
				Description: "A test category",
				Color:       "#FF5733",
			},
			wantErr: true,
		},
		{
			name: "Whitespace-only name",
			category: &Category{
				Name:  "   ",
				Color: "#FF5733",
			},
			wantErr: true,
		},
		{
			name: "Invalid color - wrong format",
			category: &Category{
				Name:        "Test Category",
				Description: "A test category",
				Color:       "#XYZ",
			},
			wantErr: true,
		},
		{
			name: "Invalid color - too long",
			category: &Category{
				Name:        "Test Category",
				Description: "A test category",
				Color:       "#FF5733FF",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.category.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Category.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check if color has been normalized with a # prefix when provided without one
			if !tt.wantErr && tt.name == "Valid category with color without hash" && tt.category.Color[0] != '#' {
				t.Errorf("Category.Validate() did not add # prefix to color, got %v", tt.category.Color)
			}

			// Check if spaces are trimmed from name
			if !tt.wantErr && tt.category.Name != "" && tt.category.Name != "Test Category" {
				t.Errorf("Category.Validate() did not trim spaces from name, got %v", tt.category.Name)
			}
		})
	}
}

func TestCategory_GetColorOrDefault(t *testing.T) {
	t.Parallel()

	t.Run("returns color when set", func(t *testing.T) {
		cat := &Category{Name: "Test", Color: "#FF5733"}
		if got := cat.GetColorOrDefault(); got != "#FF5733" {
			t.Errorf("GetColorOrDefault() = %v, want #FF5733", got)
		}
	})

	t.Run("returns default when empty", func(t *testing.T) {
		cat := &Category{Name: "Test", Color: ""}
		got := cat.GetColorOrDefault()
		if got == "" {
			t.Error("GetColorOrDefault() should return default color when empty")
		}
	})
}

func TestCategory_HasDescription(t *testing.T) {
	t.Parallel()

	t.Run("returns true when description is set", func(t *testing.T) {
		cat := &Category{Name: "Test", Description: "A description"}
		if !cat.HasDescription() {
			t.Error("HasDescription() should return true when description is set")
		}
	})

	t.Run("returns false when description is empty", func(t *testing.T) {
		cat := &Category{Name: "Test", Description: ""}
		if cat.HasDescription() {
			t.Error("HasDescription() should return false when description is empty")
		}
	})
}
