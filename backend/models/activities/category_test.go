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

func TestCategory_TableName(t *testing.T) {
	cat := &Category{}
	if got := cat.TableName(); got != "activities.categories" {
		t.Errorf("TableName() = %v, want activities.categories", got)
	}
}

func TestCategory_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		cat := &Category{Name: "Test"}
		err := cat.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		cat := &Category{Name: "Test"}
		err := cat.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestCategory_GetID(t *testing.T) {
	cat := &Category{
		TenantModel: base.TenantModel{Model: base.Model{ID: 42}},
		Name:        "Test",
	}

	if got, ok := cat.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", cat.GetID())
	}
}

func TestCategory_GetCreatedAt(t *testing.T) {
	now := time.Now()
	cat := &Category{
		TenantModel: base.TenantModel{Model: base.Model{CreatedAt: now}},
		Name:        "Test",
	}

	if got := cat.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestCategory_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	cat := &Category{
		TenantModel: base.TenantModel{Model: base.Model{UpdatedAt: now}},
		Name:        "Test",
	}

	if got := cat.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}

func TestCategory_GetColorOrDefault(t *testing.T) {
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
