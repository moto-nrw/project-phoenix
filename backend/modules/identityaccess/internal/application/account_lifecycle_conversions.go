package application

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

var errAccountLifecycleUnavailable = identityaccess.ErrAccountLifecycleUnavailable

func roleFacts(role *identityaccess.RoleFacts) *domain.RoleFacts {
	if role == nil {
		return nil
	}
	facts := domain.RoleFacts(*role)
	return &facts
}

func parentAccount(value domain.ParentAccount) identityaccess.ParentAccount {
	return identityaccess.ParentAccount{
		ID: value.ID, TenantID: value.TenantID, Email: value.Email, Username: value.Username, Active: value.Active,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func publicGuardianInvitations(invitations []domain.GuardianInvitation) []identityaccess.GuardianInvitation {
	if invitations == nil {
		return nil
	}
	result := make([]identityaccess.GuardianInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, identityaccess.GuardianInvitation(invitation))
	}
	return result
}
