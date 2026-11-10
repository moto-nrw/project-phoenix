package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/migrate"
)

func TestValidateMigrationsUsesBunExecutionOrder(t *testing.T) {
	t.Parallel()
	dependency := &Migration{Version: "1.6.17.2"}
	dependent := &Migration{Version: "1.6.17.10", DependsOn: []string{dependency.Version}}
	registry := SafeMigrationMap{dependency.Version: dependency, dependent.Version: dependent}
	bun := migrate.NewMigrations()
	bun.Add(migrate.Migration{Name: "0010060172"})
	bun.Add(migrate.Migration{Name: "00100601710"})
	metadata := map[string]*Migration{"0010060172": dependency, "00100601710": dependent}

	// Numerically 17.2 precedes 17.10, but Bun executes the latter filename first.
	err := validateMigrations(registry, bun.Sorted(), metadata)
	require.ErrorContains(t, err, "migration ordering issue")
	require.ErrorContains(t, err, "1.6.17.10 depends on 1.6.17.2")
}

func TestValidateMigrationsAcceptsTheRegisteredTree(t *testing.T) {
	t.Parallel()
	require.NoError(t, ValidateMigrations())
}

func TestValidateMigrationsPreservesDependencyValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		dependsOn []string
		wantError string
	}{
		{"earlier dependency", []string{"1.0.0"}, ""},
		{"missing dependency", []string{"0.9.0"}, "doesn't exist"},
		{"self dependency", []string{"2.0.0"}, "migration ordering issue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			first := &Migration{Version: "1.0.0"}
			second := &Migration{Version: "2.0.0", DependsOn: tc.dependsOn}
			bun := migrate.NewMigrations()
			bun.Add(migrate.Migration{Name: "001"})
			bun.Add(migrate.Migration{Name: "002"})
			// Historical Bun-only migrations do not have dependency metadata.
			bun.Add(migrate.Migration{Name: "000"})
			err := validateMigrations(SafeMigrationMap{first.Version: first, second.Version: second}, bun.Sorted(), map[string]*Migration{"001": first, "002": second})
			if tc.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantError)
			}
		})
	}
}
