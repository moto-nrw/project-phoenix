//go:build darwin || linux

package testdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const corruptStartLog = `LOG:  invalid record length at 1/5051C3F8: expected at least 24, got 0
LOG:  invalid checkpoint record
PANIC:  could not locate a valid checkpoint record at 1/5051C3F8
`

type corruptFixture struct {
	container idleContainer
	logs      map[string]string // keyed by the --since value
	removed   []string
}

func newCorruptFixture(t *testing.T) (*corruptFixture, idleServer) {
	t.Helper()
	project := composeProjectPrefix + "-6543"
	f := &corruptFixture{logs: map[string]string{"2026-11-08T10:00:00Z": corruptStartLog}}
	c := &f.container
	c.ID = "corrupt"
	c.Config.Labels = map[string]string{idleLabel: "1", "com.docker.compose.project": project, "com.docker.compose.service": "postgres-test"}
	c.State.ExitCode = 1
	c.State.StartedAt = "2026-11-08T10:00:00Z"
	require.NoError(t, json.Unmarshal([]byte(`{"5432/tcp":[{"HostPort":"6543"}]}`), &c.HostConfig.PortBindings))
	require.NoError(t, json.Unmarshal([]byte(`[{"Type":"volume","Name":"`+project+`_postgres_test","Destination":"/var/lib/postgresql/data"}]`), &c.Mounts))
	server := idleServer{Host: "unix:///fixture.sock", Project: project, Docker: "/fixture/docker", run: func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) < 3 || args[0] != "--host" || args[1] != "unix:///fixture.sock" {
			return nil, fmt.Errorf("Docker endpoint was not pinned")
		}
		args = args[2:]
		switch args[0] {
		case "ps":
			return []byte(f.container.ID + "\n"), nil
		case "inspect":
			return json.Marshal([]idleContainer{f.container})
		case "logs":
			require.Equal(t, []string{"logs", "--since", f.container.State.StartedAt, f.container.ID}, args)
			return []byte(f.logs[args[2]]), nil
		case "rm", "volume":
			f.removed = append(f.removed, strings.Join(args, " "))
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected Docker command: %s", args[0])
		}
	}}
	return f, server
}

func TestRebuildRemovesManagedServerThatCannotFindItsCheckpoint(t *testing.T) {
	t.Parallel()
	f, server := newCorruptFixture(t)

	rebuilt, err := server.removeCorrupt(context.Background())

	require.NoError(t, err)
	assert.True(t, rebuilt)
	assert.Equal(t, []string{"rm --force corrupt", "volume rm " + server.Project + "_postgres_test"}, f.removed)
}

func TestRebuildLeavesOtherServerStatesAlone(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*corruptFixture){
		"running":                   func(f *corruptFixture) { f.container.State.Running = true },
		"clean idle stop":           func(f *corruptFixture) { f.container.State.ExitCode = 0 },
		"other startup failure":     func(f *corruptFixture) { f.logs[f.container.State.StartedAt] = "FATAL:  could not write lock file\n" },
		"marker from earlier start": func(f *corruptFixture) { f.container.State.StartedAt = "2026-11-08T11:00:00Z" },
		"unmanaged container":       func(f *corruptFixture) { delete(f.container.Config.Labels, idleLabel) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, server := newCorruptFixture(t)
			mutate(f)

			rebuilt, err := server.removeCorrupt(context.Background())

			require.NoError(t, err)
			assert.False(t, rebuilt)
			assert.Empty(t, f.removed)
		})
	}
}

func TestRebuildRefusesUnexpectedDataVolume(t *testing.T) {
	t.Parallel()
	f, server := newCorruptFixture(t)
	f.container.Mounts[0].Name = "project-phoenix_postgres_data"

	rebuilt, err := server.removeCorrupt(context.Background())

	require.ErrorContains(t, err, "unexpected data volume")
	assert.False(t, rebuilt)
	assert.Empty(t, f.removed)
}
