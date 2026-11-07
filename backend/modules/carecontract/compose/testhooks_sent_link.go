//go:build testhooks

package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/application"
)

// SendLinkForTests stores the signing link a send of the contract makes
// (#4679), in the caller's Träger transaction.
func SendLinkForTests(ctx context.Context, links carecontract.SigningLinks, organizationID, contractID int64) error {
	return application.SendLinkForTests(ctx, postgres.NewStore(), links, organizationID, contractID)
}
