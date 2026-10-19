//go:build testhooks

package application

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// Preserve the concrete fixture facet while arranging the same application
// transaction used by production guardian relationship writes.
func (c guardianRelationshipCommands) CreateGuardianLinkFixture(ctx context.Context, value *peopledirectory.GuardianLinkFixture) error {
	// Preserve fixture validation before acquiring a database unit of work.
	if value == nil || value.StudentID <= 0 || value.GuardianProfileID <= 0 {
		return errors.New("guardian relationship fixture requires a student and guardian")
	}
	if len(value.Permissions) != 0 {
		var permissions map[string]any
		if err := json.Unmarshal(value.Permissions, &permissions); err != nil {
			return err
		}
	}
	writer, ok := c.StudentGuardianRepository.(peopledirectory.GuardianLinkFixtureWriter)
	if !ok {
		return errors.New("guardian relationship fixture writer is unavailable")
	}
	return c.transactions.InTenantTx(ctx, func(txCtx context.Context) error { return writer.CreateGuardianLinkFixture(txCtx, value) })
}
