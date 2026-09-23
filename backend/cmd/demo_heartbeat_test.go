package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The Compose healthcheck reads the heartbeat's modification time, so every
// beat has to move it forward, including over an existing file.
func TestDemoHeartbeatRefreshesTheFileOnEveryBeat(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "demo-heartbeat")
	if err := writeDemoHeartbeat(path); err != nil {
		t.Fatalf("first beat: %v", err)
	}
	stale := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, stale, stale); err != nil {
		t.Fatal(err)
	}
	if err := writeDemoHeartbeat(path); err != nil {
		t.Fatalf("second beat: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(info.ModTime()) > time.Minute {
		t.Fatalf("heartbeat stayed stale: %v", info.ModTime())
	}
}

func TestDemoHeartbeatIsOptionalAndReportsUnwritablePaths(t *testing.T) {
	t.Parallel()
	if err := writeDemoHeartbeat(""); err != nil {
		t.Fatalf("no path means no heartbeat, got %v", err)
	}
	if err := writeDemoHeartbeat(filepath.Join(t.TempDir(), "missing", "demo-heartbeat")); err == nil {
		t.Fatal("an unwritable heartbeat path must be reported")
	}
}
