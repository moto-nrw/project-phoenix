package models

import (
	"errors"
	"net/http"
	"time"

	"github.com/uptrace/bun"
)

// Setting represents a system configuration setting
type Setting struct {
	bun.BaseModel `bun:"table:settings"`

	ID              int64     `json:"id" bun:"id,pk,autoincrement"`
	Key             string    `json:"key" bun:"key,unique,notnull"`
	Value           string    `json:"value" bun:"value,notnull"`
	Category        string    `json:"category" bun:"category,notnull"`
	Description     string    `json:"description" bun:"description"`
	RequiresRestart bool      `json:"requires_restart" bun:"requires_restart,notnull,default:false"`
	RequiresDBReset bool      `json:"requires_db_reset" bun:"requires_db_reset,notnull,default:false"`
	CreatedAt       time.Time `json:"created_at" bun:"created_at,notnull,default:current_timestamp"`
	ModifiedAt      time.Time `json:"modified_at" bun:"modified_at,notnull,default:current_timestamp"`
}

// BeforeUpdate hook updates the ModifiedAt timestamp
func (s *Setting) BeforeUpdate(ctx bun.BeforeUpdateHook) error {
	s.ModifiedAt = time.Now()
	return nil
}

// SettingRequest represents a request to create or update a setting
type SettingRequest struct {
	Key             string `json:"key"`
	Value           string `json:"value"`
	Category        string `json:"category"`
	Description     string `json:"description,omitempty"`
	RequiresRestart bool   `json:"requires_restart,omitempty"`
	RequiresDBReset bool   `json:"requires_db_reset,omitempty"`
}

// Bind binds and validates the request
func (s *SettingRequest) Bind(r *http.Request) error {
	if s.Key == "" {
		return errors.New("Key is required")
	}
	if s.Value == "" {
		return errors.New("Value is required")
	}
	if s.Category == "" {
		return errors.New("Category is required")
	}
	return nil
}

// SettingResponse represents a response containing setting data
type SettingResponse struct {
	ID              int64     `json:"id"`
	Key             string    `json:"key"`
	Value           string    `json:"value"`
	Category        string    `json:"category"`
	Description     string    `json:"description,omitempty"`
	RequiresRestart bool      `json:"requires_restart,omitempty"`
	RequiresDBReset bool      `json:"requires_db_reset,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	ModifiedAt      time.Time `json:"modified_at,omitempty"`
}
