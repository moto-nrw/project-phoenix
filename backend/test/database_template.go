package test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/moto-ogs/project-phoenix/internal/testdb"
	"github.com/uptrace/bun"
)

// IsolatedTemplate is a package-local template for isolated clones. Its build
// runs once per test binary, on a clone of the run's template; every test that
// asks for it gets a disposable clone of the result, as isolated from other
// tests as one from SetupIsolatedTestDB. A test may change its clone in any
// way, including its schema.
//
// It exists for fixtures whose construction dominates their tests: the tenant
// export's schema-shaped fixture issued about 960 DDL statements per test
// (#6962).
type IsolatedTemplate struct {
	name  string
	build func(ctx context.Context, db *bun.DB) error

	once  sync.Once
	clone *testdb.CloneHandle
	err   error
}

// NewIsolatedTemplate declares a package-local template. name tells the
// templates of one package apart; build prepares it and must not depend on
// the test that happens to trigger it.
func NewIsolatedTemplate(name string, build func(ctx context.Context, db *bun.DB) error) *IsolatedTemplate {
	return &IsolatedTemplate{name: name, build: build}
}

// SetupIsolatedTestDBFrom is SetupIsolatedTestDB with a clone of template
// instead of the run's template. The clone carries no tenant rows of the test:
// the template's build owns its schema.
func SetupIsolatedTestDBFrom(tb testing.TB, template *IsolatedTemplate) *bun.DB {
	tb.Helper()
	setupPackageTestDB(tb)
	return setupIsolatedTestDB(tb, template)
}

var (
	builtIsolatedTemplatesMu sync.Mutex
	builtIsolatedTemplates   []*testdb.CloneHandle
)

// built returns the template's clone, building it on first use.
func (template *IsolatedTemplate) built(ctx context.Context) (*testdb.CloneHandle, error) {
	template.once.Do(func() {
		template.clone, template.err = buildIsolatedTemplate(ctx, template)
	})
	return template.clone, template.err
}

func buildIsolatedTemplate(ctx context.Context, template *IsolatedTemplate) (*testdb.CloneHandle, error) {
	clone, err := packageMaintenance.CreateClone(ctx, packageCloneCfg, testdb.RunID()+"-template-"+template.name)
	if err != nil {
		return nil, fmt.Errorf("create package template %q: %w", template.name, err)
	}
	db, err := openTestPool(ctx, clone.DSN)
	if err == nil {
		err = template.build(ctx, db)
		// Copying needs the template without sessions.
		err = errors.Join(err, db.Close())
	}
	if err != nil {
		err = errors.Join(err, clone.Close(), packageMaintenance.DropClone(ctx, clone.Name))
		return nil, fmt.Errorf("build package template %q: %w", template.name, err)
	}
	builtIsolatedTemplatesMu.Lock()
	builtIsolatedTemplates = append(builtIsolatedTemplates, clone)
	builtIsolatedTemplatesMu.Unlock()
	return clone, nil
}

// dropIsolatedTemplates drops the package-local templates at the end of the
// binary; a killed binary leaves them to the generation GC like its clones.
func dropIsolatedTemplates(ctx context.Context) error {
	builtIsolatedTemplatesMu.Lock()
	defer builtIsolatedTemplatesMu.Unlock()
	var err error
	for _, clone := range builtIsolatedTemplates {
		err = errors.Join(err, clone.Close(), packageMaintenance.DropClone(ctx, clone.Name))
	}
	builtIsolatedTemplates = nil
	return err
}
