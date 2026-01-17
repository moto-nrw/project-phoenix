package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func runCleanupInvitations(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithServices()
	if err != nil {
		return err
	}
	defer ctx.Close()

	if ctx.ServiceFactory.Invitation == nil {
		fmt.Println("Invitation service is not available; nothing to clean.")
		return nil
	}

	count, err := ctx.ServiceFactory.Invitation.CleanupExpiredInvitations(context.Background())
	if err != nil {
		return fmt.Errorf("failed to clean up invitations: %w", err)
	}

	fmt.Printf("Invitation cleanup removed %d records\n", count)
	return nil
}
