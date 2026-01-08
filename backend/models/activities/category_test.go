package activities

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
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

func TestCategoryGetColorOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		category *Category
		want     string
	}{
		{
			name: "With color",
			category: &Category{
				Color: "#FF5733",
			},
			want: "#FF5733",
		},
		{
			name:     "Without color",
			category: &Category{},
			want:     "#CCCCCC", // Default color
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.category.GetColorOrDefault(); got != tt.want {
				t.Errorf("Category.GetColorOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCategoryHasDescription(t *testing.T) {
	tests := []struct {
		name     string
		category *Category
		want     bool
	}{
		{
			name: "With description",
			category: &Category{
				Description: "A test category",
			},
			want: true,
		},
		{
			name:     "Without description",
			category: &Category{},
			want:     false,
		},
		{
			name: "With empty description",
			category: &Category{
				Description: "",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.category.HasDescription(); got != tt.want {
				t.Errorf("Category.HasDescription() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCategoryTableName(t *testing.T) {
	category := &Category{}
	expected := "activities.categories"

	if got := category.TableName(); got != expected {
		t.Errorf("Category.TableName() = %v, want %v", got, expected)
	}
}

func TestCategory_EntityInterface(t *testing.T) {
	now := time.Now()
	category := &Category{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		Name: "Test Category",
	}

	t.Run("GetID", func(t *testing.T) {
		got := category.GetID()
		if got != int64(123) {
			t.Errorf("Category.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := category.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Category.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := category.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Category.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
