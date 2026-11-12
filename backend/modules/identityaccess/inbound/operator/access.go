package operator

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

// OperatorAccess is what the invitation, profile and second-factor routes
// need: the operator directory, the token mint their second factors end in,
// the invitation and e-mail change flows, and the account deactivation.
type OperatorAccess interface {
	identityaccess.OperatorDirectory
	IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, credentialGeneration *int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	identityaccess.OperatorProvisioning
	identityaccess.OperatorStatus
}

// InvalidInput reports input the owner refused and hands back the message
// the operator surface translates into German. It is the only refusal whose
// text the surface reads.
func InvalidInput(err error) (error, bool) {
	var invalid *identityaccess.InvalidInputError
	if errors.As(err, &invalid) {
		return invalid.Err, true
	}
	return nil, false
}
