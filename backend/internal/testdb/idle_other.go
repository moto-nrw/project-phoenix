//go:build !darwin && !linux

package testdb

import (
	"context"
	"fmt"
	"os"
	"time"
)

// HoldServer leaves unsupported hosts unchanged; use WSL for local cleanup.
func HoldServer(context.Context, *Config) error { return nil }
func WatchIdle(context.Context, string, *os.File, time.Duration, time.Duration) error {
	return fmt.Errorf("automatic test server cleanup requires macOS or Linux (including WSL)")
}
func rebuildCorruptServer(context.Context, *Config) (bool, error) { return false, nil }
func startTestContainerLocked(ctx context.Context, cfg *Config) error {
	return startTestContainer(ctx, cfg)
}
func IdleStatus(context.Context) error {
	return fmt.Errorf("automatic test server cleanup requires macOS or Linux (including WSL)")
}
