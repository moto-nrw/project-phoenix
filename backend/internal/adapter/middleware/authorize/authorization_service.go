package authorize

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/policy"
)

// AuthorizationService provides resource-specific authorization
type AuthorizationService interface {
	// AuthorizeResource checks if a subject can perform an action on a specific resource
	AuthorizeResource(ctx context.Context, subject policy.Subject, resource policy.Resource, action policy.Action, extra map[string]interface{}) (bool, error)

	// RegisterPolicy registers a new authorization policy
	RegisterPolicy(policy policy.Policy) error

	// GetPolicyEngine returns the underlying policy engine
	GetPolicyEngine() policy.PolicyEngine
}

// authorizationService implements the AuthorizationService interface
type authorizationService struct {
	policyEngine policy.PolicyEngine
}

// NewAuthorizationService creates a new authorization service
func NewAuthorizationService() AuthorizationService {
	return &authorizationService{
		policyEngine: policy.NewPolicyEngine(),
	}
}

// AuthorizeResource checks if a subject can perform an action on a specific resource
func (s *authorizationService) AuthorizeResource(ctx context.Context, subject policy.Subject, resource policy.Resource, action policy.Action, extra map[string]interface{}) (bool, error) {
	authCtx := &policy.Context{
		Subject:  subject,
		Resource: resource,
		Action:   action,
		Extra:    extra,
	}

	return s.policyEngine.Authorize(ctx, authCtx)
}

// RegisterPolicy registers a new authorization policy
func (s *authorizationService) RegisterPolicy(p policy.Policy) error {
	return s.policyEngine.RegisterPolicy(p)
}

// GetPolicyEngine returns the underlying policy engine
func (s *authorizationService) GetPolicyEngine() policy.PolicyEngine {
	return s.policyEngine
}
