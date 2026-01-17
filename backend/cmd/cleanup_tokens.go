package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func runCleanupTokens(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithServices()
	if err != nil {
		return err
	}
	defer ctx.Close()

	count, err := countExpiredTokens(ctx)
	if err != nil {
		return fmt.Errorf("failed to count expired tokens: %w", err)
	}

	fmt.Printf("Found %d expired tokens to clean up\n", count)

	if count == 0 {
		fmt.Println("No expired tokens to clean up")
		return nil
	}

	deletedCount, err := ctx.ServiceFactory.Auth.CleanupExpiredTokens(context.Background())
	if err != nil {
		return fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	fmt.Printf("Successfully deleted %d expired tokens\n", deletedCount)
	return nil
}

func countExpiredTokens(ctx *cleanupContext) (int, error) {
	return ctx.DB.NewSelect().
		TableExpr("auth.tokens").
		Where("expiry < ?", time.Now()).
		Count(context.Background())
}
