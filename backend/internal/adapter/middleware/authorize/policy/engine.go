package policy

import (
	"context"
	"fmt"
	"sync"
)

// DefaultPolicyEngine is the default implementation of PolicyEngine
type DefaultPolicyEngine struct {
	policies         map[string]Policy
	resourceToPolicy map[string][]Policy // Maps resource types to policies
	mu               sync.RWMutex
}

// NewPolicyEngine creates a new policy engine
func NewPolicyEngine() PolicyEngine {
	return &DefaultPolicyEngine{
		policies:         make(map[string]Policy),
		resourceToPolicy: make(map[string][]Policy),
	}
}

// RegisterPolicy registers a new policy
func (e *DefaultPolicyEngine) RegisterPolicy(policy Policy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.policies[policy.Name()]; exists {
		return fmt.Errorf("policy %s already registered", policy.Name())
	}

	// Register the policy by name
	e.policies[policy.Name()] = policy

	// Also maintain a map by resource type for faster lookup
	resourceType := policy.ResourceType()
	e.resourceToPolicy[resourceType] = append(e.resourceToPolicy[resourceType], policy)

	return nil
}

// Authorize evaluates whether a subject can perform an action on a resource
func (e *DefaultPolicyEngine) Authorize(ctx context.Context, authCtx *Context) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Find all policies for this resource type
	relevantPolicies := e.resourceToPolicy[authCtx.Resource.Type]

	// If no policies found for this resource type, deny by default
	if len(relevantPolicies) == 0 {
		return false, nil
	}

	// Evaluate each policy - all must pass
	for _, policy := range relevantPolicies {
		allowed, err := policy.Evaluate(ctx, authCtx)
		if err != nil {
			return false, fmt.Errorf("policy %s evaluation failed: %w", policy.Name(), err)
		}

		if !allowed {
			return false, nil
		}
	}

	return true, nil
}

// GetPoliciesForResource returns all policies for a specific resource type
func (e *DefaultPolicyEngine) GetPoliciesForResource(resourceType string) []Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.resourceToPolicy[resourceType]
}
