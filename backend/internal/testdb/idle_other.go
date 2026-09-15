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
func IdleStatus(context.Context) error {
	return fmt.Errorf("automatic test server cleanup requires macOS or Linux (including WSL)")
}
