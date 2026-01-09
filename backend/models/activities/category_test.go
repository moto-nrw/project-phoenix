package activities

import (
	"testing"
)

func TestCategoryValidate(t *testing.T) {
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
