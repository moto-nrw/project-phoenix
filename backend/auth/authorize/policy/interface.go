package policy

import (
	"context"
)

// Action represents an action that can be performed on a resource
type Action string

const (
	ActionView   Action = "view"
	ActionEdit   Action = "edit"
	ActionCreate Action = "create"
	ActionDelete Action = "delete"
	ActionManage Action = "manage"
)

// Resource represents a resource that can be accessed
type Resource struct {
	Type string      // e.g., "student", "visit", "group"
	ID   interface{} // The ID of the specific resource
}

// Subject represents the user making the request
type Subject struct {
	AccountID   int64
	Roles       []string
	Permissions []string
}

// Context represents additional context for authorization decisions
type Context struct {
	Resource Resource
	Action   Action
	Subject  Subject
	Extra    map[string]interface{} // Additional context data
}

// Policy defines a single authorization policy
type Policy interface {
	// Name returns the name of the policy
	Name() string

	// ResourceType returns the resource type this policy applies to
	ResourceType() string

	// Evaluate evaluates whether the subject can perform the action on the resource
	Evaluate(ctx context.Context, authCtx *Context) (bool, error)
}

// PolicyEngine manages and evaluates authorization policies
type PolicyEngine interface {
	// RegisterPolicy registers a new policy
	RegisterPolicy(policy Policy) error

	// Authorize evaluates whether a subject can perform an action on a resource
	Authorize(ctx context.Context, authCtx *Context) (bool, error)

	// GetPoliciesForResource returns all policies for a specific resource type
	GetPoliciesForResource(resourceType string) []Policy
}
