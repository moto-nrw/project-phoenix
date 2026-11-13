package application

import (
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

func NewReviewPermissions(resolve func([]string) identityaccess.ReviewPermissions) func(permissions []string) domain.ReviewPermissions {
	return func(permissions []string) domain.ReviewPermissions {
		return domain.ReviewPermissions(resolve(permissions))
	}
}
