package config

import (
	"context"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// ActionHandler is a function that executes an action
type ActionHandler func(ctx context.Context, audit *config.ActionAuditContext) (*ActionResult, error)

// ActionResult represents the result of an action execution
type ActionResult struct {
	// Success indicates if the action completed successfully
	Success bool `json:"success"`
	// Message is a human-readable result message
	Message string `json:"message"`
	// Data contains optional additional result data
	Data interface{} `json:"data,omitempty"`
}

// ActionRegistry manages action handlers
type ActionRegistry struct {
	handlers map[string]ActionHandler
	mu       sync.RWMutex
}

// Global action registry
var globalActionRegistry = &ActionRegistry{
	handlers: make(map[string]ActionHandler),
}

// RegisterActionHandler registers a handler for an action key
func RegisterActionHandler(key string, handler ActionHandler) {
	globalActionRegistry.mu.Lock()
	defer globalActionRegistry.mu.Unlock()
	globalActionRegistry.handlers[key] = handler
}

// GetActionHandler retrieves a handler for an action key
func GetActionHandler(key string) ActionHandler {
	globalActionRegistry.mu.RLock()
	defer globalActionRegistry.mu.RUnlock()
	return globalActionRegistry.handlers[key]
}

// HasActionHandler checks if a handler is registered for an action key
func HasActionHandler(key string) bool {
	globalActionRegistry.mu.RLock()
	defer globalActionRegistry.mu.RUnlock()
	_, exists := globalActionRegistry.handlers[key]
	return exists
}

// ListActionHandlers returns all registered action keys
func ListActionHandlers() []string {
	globalActionRegistry.mu.RLock()
	defer globalActionRegistry.mu.RUnlock()

	keys := make([]string, 0, len(globalActionRegistry.handlers))
	for key := range globalActionRegistry.handlers {
		keys = append(keys, key)
	}
	return keys
}

// ExecuteAction executes an action and records the result in the audit log
func (s *HierarchicalSettingsServiceImpl) ExecuteAction(ctx context.Context, key string, audit *config.ActionAuditContext) (*ActionResult, error) {
	handler := GetActionHandler(key)
	if handler == nil {
		return &ActionResult{
			Success: false,
			Message: "Kein Handler für diese Aktion registriert",
		}, nil
	}

	startTime := time.Now()
	result, err := handler(ctx, audit)
	duration := time.Since(startTime).Milliseconds()

	// Record in audit log
	var errorMsg string
	var resultSummary string
	var success bool

	if err != nil {
		errorMsg = err.Error()
		success = false
	} else if result != nil {
		success = result.Success
		resultSummary = result.Message
		if !success && result.Message != "" {
			errorMsg = result.Message
		}
	}

	auditEntry := audit.ToAuditEntry(key, success, duration, errorMsg, resultSummary)
	if s.actionAuditRepo != nil {
		_ = s.actionAuditRepo.Create(ctx, auditEntry) // Fire and forget
	}

	if err != nil {
		return &ActionResult{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return result, nil
}

// GetActionHistory retrieves the execution history for an action
func (s *HierarchicalSettingsServiceImpl) GetActionHistory(ctx context.Context, key string, limit int) ([]*config.ActionAuditEntryDTO, error) {
	if s.actionAuditRepo == nil {
		return nil, nil
	}

	entries, err := s.actionAuditRepo.FindByActionKey(ctx, key, limit)
	if err != nil {
		return nil, err
	}

	dtos := make([]*config.ActionAuditEntryDTO, len(entries))
	for i, entry := range entries {
		dtos[i] = entry.ToDTO()
	}
	return dtos, nil
}

// GetRecentActionExecutions retrieves recent action executions across all actions
func (s *HierarchicalSettingsServiceImpl) GetRecentActionExecutions(ctx context.Context, limit int) ([]*config.ActionAuditEntryDTO, error) {
	if s.actionAuditRepo == nil {
		return nil, nil
	}

	entries, err := s.actionAuditRepo.FindRecent(ctx, limit)
	if err != nil {
		return nil, err
	}

	dtos := make([]*config.ActionAuditEntryDTO, len(entries))
	for i, entry := range entries {
		dtos[i] = entry.ToDTO()
	}
	return dtos, nil
}
