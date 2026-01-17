package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func runCleanupRateLimits(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithServices()
	if err != nil {
		return err
	}
	defer ctx.Close()

	count, err := ctx.ServiceFactory.Auth.CleanupExpiredRateLimits(context.Background())
	if err != nil {
		return fmt.Errorf("failed to clean up password reset rate limits: %w", err)
	}

	fmt.Printf("Password reset rate limit cleanup removed %d records\n", count)
	return nil
}
