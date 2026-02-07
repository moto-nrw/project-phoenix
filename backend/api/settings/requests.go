package settings

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// SetValueRequest represents a request to set a setting value
type SetValueRequest struct {
	Value   string       `json:"value"`
	Scope   config.Scope `json:"scope"`
	ScopeID *int64       `json:"scope_id,omitempty"`
}

// Bind validates the set value request
func (req *SetValueRequest) Bind(_ *http.Request) error {
	if !req.Scope.IsValid() {
		return errors.New("invalid scope")
	}
	if req.Scope == config.ScopeSystem && req.ScopeID != nil {
		return errors.New("scope_id must be null for system scope")
	}
	if req.Scope != config.ScopeSystem && req.ScopeID == nil {
		return errors.New("scope_id is required for non-system scope")
	}
	return nil
}

// DeleteValueRequest represents a request to delete a setting override
type DeleteValueRequest struct {
	Scope   config.Scope `json:"scope"`
	ScopeID *int64       `json:"scope_id,omitempty"`
}

// Bind validates the delete value request
func (req *DeleteValueRequest) Bind(_ *http.Request) error {
	if !req.Scope.IsValid() {
		return errors.New("invalid scope")
	}
	if req.Scope == config.ScopeSystem && req.ScopeID == nil {
		return errors.New("cannot delete system default")
	}
	return nil
}

// RestoreValueRequest represents a request to restore a soft-deleted value
type RestoreValueRequest struct {
	Scope   config.Scope `json:"scope"`
	ScopeID *int64       `json:"scope_id,omitempty"`
}

// Bind validates the restore value request
func (req *RestoreValueRequest) Bind(_ *http.Request) error {
	if !req.Scope.IsValid() {
		return errors.New("invalid scope")
	}
	return nil
}

// PurgeRequest represents a request to purge old deleted records
type PurgeRequest struct {
	Days int `json:"days"`
}

// Bind validates the purge request
func (req *PurgeRequest) Bind(_ *http.Request) error {
	if req.Days < 0 {
		return errors.New("days must be non-negative")
	}
	if req.Days > 365 {
		return errors.New("days must be at most 365")
	}
	return nil
}
