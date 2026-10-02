package compose

import (
	"context"
	"errors"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// TenantSnapshot is one school's settings resolved ahead of use. Bind returns
// a context whose settings reads the snapshot serves.
type TenantSnapshot interface {
	Bind(ctx context.Context) context.Context
}

// TenantSnapshots resolves the named settings of many schools in one read.
type TenantSnapshots func(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]TenantSnapshot, error)

var errBatchUnsupported = errors.New("settings service does not resolve many schools at once")

// NewTenantSnapshots binds the cross-tenant batch read of the retained
// settings service, for consumers that preload the settings of every school
// at once, such as the Worker's minute snapshot (#2746).
func NewTenantSnapshots(service configSvc.SettingsService) (TenantSnapshots, error) {
	batch, ok := service.(configSvc.BatchSettingsService)
	if !ok {
		return nil, errBatchUnsupported
	}
	return func(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]TenantSnapshot, error) {
		resolved, err := batch.ResolveManyForTenants(ctx, tenantIDs, keys)
		if err != nil {
			return nil, err
		}
		snapshots := make(map[int64]TenantSnapshot, len(resolved))
		for tenantID, snapshot := range resolved {
			if snapshot != nil {
				snapshots[tenantID] = tenantSnapshot{snapshot: snapshot}
			}
		}
		return snapshots, nil
	}, nil
}

type tenantSnapshot struct {
	snapshot *configSvc.SettingsSnapshot
}

func (s tenantSnapshot) Bind(ctx context.Context) context.Context {
	return configSvc.WithSettingsSnapshot(ctx, s.snapshot)
}

// VerifyPreloadKeys refuses keys a consumer preloads for every school when
// the registry does not define them or defines them as secrets. An unknown
// key answers its fallback on every read, and a secret must not sit in a
// long-lived snapshot.
func VerifyPreloadKeys(keys []string) error {
	var problems []error
	for _, key := range keys {
		definition := configModel.GetDefinition(key)
		switch {
		case definition == nil:
			problems = append(problems, fmt.Errorf("setting %q is not registered", key))
		case definition.Type == configModel.FieldPassword:
			problems = append(problems, fmt.Errorf("setting %q is a secret", key))
		}
	}
	return errors.Join(problems...)
}
