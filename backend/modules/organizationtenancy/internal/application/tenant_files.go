package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

var (
	// errTenantFiles marks a storage failure of one namespace. The storage
	// error itself names no file; callers say what it means for their run.
	errTenantFiles = errors.New("tenant files failed")
	// errFilesRemain marks objects left after a removal.
	errFilesRemain = errors.New("tenant files remain")
)

// tenantFiles counts or removes every catalog namespace of one school. before
// runs ahead of each namespace; the school erasure checks its lock there. An
// error of before is returned as it is.
func tenantFiles(ctx context.Context, files organizationtenancy.ErasureFiles, id int64, kinds []organizationtenancy.LifecycleFileKind, remove bool, before func(context.Context) error) (map[string]int64, error) {
	counts := make(map[string]int64, len(kinds))
	for _, kind := range kinds {
		if before != nil {
			if err := before(ctx); err != nil {
				return nil, err
			}
		}
		// Avatars keep their global objects; only the legacy school
		// directories avatars/<school>/ belong to the school.
		layout := kind.Layout
		if kind.Kind == "avatars" {
			layout = "tenantDir"
		}
		n, err := files.TenantObjects(ctx, kind.Kind, layout, id, remove)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errTenantFiles, kind.Kind)
		}
		counts[kind.Kind] = n
	}
	return counts, nil
}

// removeAndVerify removes every namespace, then counts again. It returns the
// removed counts; any object left is errFilesRemain.
func removeAndVerify(phase func(remove bool) (map[string]int64, error)) (map[string]int64, error) {
	removed, err := phase(true)
	if err != nil {
		return nil, err
	}
	left, err := phase(false)
	if err != nil {
		return nil, err
	}
	for _, n := range left {
		if n != 0 {
			return nil, errFilesRemain
		}
	}
	return removed, nil
}
