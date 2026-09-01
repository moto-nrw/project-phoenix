package platform_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestAnnouncementService_CreateAnnouncement_NilAnnouncement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.CreateAnnouncement(ctx, nil, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestAnnouncementService_CreateAnnouncement_ValidationError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcement := &platform.Announcement{
		Title:   "",
		Content: "Content",
	}

	err := service.CreateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

// --- Helper to reduce service construction boilerplate ---

type testAnnouncementMocks struct {
	announcementRepo *mockAnnouncementRepoShared
	viewRepo         *mockAnnouncementViewRepoShared
	auditLogRepo     *mockAuditLogRepoShared
	orgRepo          *mockOrgRepoShared
	schoolRepo       *testpkg.SchoolRepoMock
}

func newTestAnnouncementService(overrides func(m *testAnnouncementMocks)) platformSvc.AnnouncementService {
	m := &testAnnouncementMocks{
		announcementRepo: &mockAnnouncementRepoShared{},
		viewRepo:         &mockAnnouncementViewRepoShared{},
		auditLogRepo:     &mockAuditLogRepoShared{},
		orgRepo:          &mockOrgRepoShared{},
		schoolRepo: &testpkg.SchoolRepoMock{
			// Matches mockSchoolRepoShared's old default: an unconfigured
			// CountByIDs reports every requested ID as existing, so tests
			// that only care about org targeting don't need to stub it.
			CountByIDsFn: func(_ context.Context, ids []int64) (int, error) {
				return len(ids), nil
			},
		},
	}
	if overrides != nil {
		overrides(m)
	}
	return platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     m.announcementRepo,
		AnnouncementViewRepo: m.viewRepo,
		AuditLogRepo:         m.auditLogRepo,
		Organizations:        m.orgRepo,
		SchoolRepo:           m.schoolRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})
}

func validAnnouncement() *platform.Announcement {
	return &platform.Announcement{
		Title:    "Test",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}
}

func existingAnnouncement() *platform.Announcement {
	return &platform.Announcement{
		Model:    base.Model{ID: 1},
		Title:    "Test",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}
}

// --- Targeting: nil normalization ---

func TestAnnouncementService_CreateAnnouncement_NilTargetingNormalized(t *testing.T) {
	t.Parallel()

	var captured *platform.Announcement
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.createFn = func(_ context.Context, a *platform.Announcement) error {
			captured = a
			a.ID = 100
			return nil
		}
	})

	a := validAnnouncement()
	a.TargetOrgIDs = nil
	a.TargetTenantIDs = nil

	err := svc.CreateAnnouncement(context.Background(), a, 42, nil)
	require.NoError(t, err)
	assert.NotNil(t, captured.TargetOrgIDs, "nil TargetOrgIDs should be normalized to empty slice by Validate")
	assert.NotNil(t, captured.TargetTenantIDs, "nil TargetTenantIDs should be normalized to empty slice by Validate")
	assert.Empty(t, captured.TargetOrgIDs)
	assert.Empty(t, captured.TargetTenantIDs)
}

// --- Update: validation error, targeting, audit log ---

func TestAnnouncementService_UpdateAnnouncement_ValidationError(t *testing.T) {
	t.Parallel()

	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
			return existingAnnouncement(), nil
		}
	})
	a := &platform.Announcement{Title: "", Content: "Content", Type: platform.TypeAnnouncement, Severity: platform.SeverityInfo}
	a.CreatedBy = 1
	err := svc.UpdateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestAnnouncementService_UpdateAnnouncement_AuditLogTargetingChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		existing     func() *platform.Announcement
		updated      func() *platform.Announcement
		wantOrgChg   bool
		wantTntChg   bool
		wantRolesChg bool
	}{
		{
			name: "targeting changed",
			existing: func() *platform.Announcement {
				a := existingAnnouncement()
				a.TargetRoles = []string{"admin"}
				a.TargetOrgIDs = []int64{1}
				a.TargetTenantIDs = []int64{10}
				a.CreatedBy = 1
				return a
			},
			updated: func() *platform.Announcement {
				a := validAnnouncement()
				a.TargetRoles = []string{"admin", "user"}
				a.TargetOrgIDs = []int64{1, 2}
				a.TargetTenantIDs = []int64{20}
				a.CreatedBy = 1
				return a
			},
			wantOrgChg:   true,
			wantTntChg:   true,
			wantRolesChg: true,
		},
		{
			name: "targeting unchanged",
			existing: func() *platform.Announcement {
				a := existingAnnouncement()
				a.TargetRoles = []string{"admin"}
				a.TargetOrgIDs = []int64{5}
				a.TargetTenantIDs = []int64{}
				a.CreatedBy = 1
				return a
			},
			updated: func() *platform.Announcement {
				a := validAnnouncement()
				a.TargetRoles = []string{"admin"}
				a.TargetOrgIDs = []int64{5}
				a.TargetTenantIDs = []int64{}
				a.CreatedBy = 1
				return a
			},
			wantOrgChg:   false,
			wantTntChg:   false,
			wantRolesChg: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedAudit *platform.OperatorAuditLog
			svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
				m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
					return tt.existing(), nil
				}
				m.announcementRepo.updateFn = func(context.Context, *platform.Announcement) error { return nil }
				m.auditLogRepo.createFn = func(_ context.Context, entry *platform.OperatorAuditLog) error {
					capturedAudit = entry
					return nil
				}
			})

			err := svc.UpdateAnnouncement(context.Background(), tt.updated(), 1, net.ParseIP("127.0.0.1"))
			require.NoError(t, err)
			require.NotNil(t, capturedAudit)

			changes, err := capturedAudit.GetChanges()
			require.NoError(t, err)
			assert.Equal(t, tt.wantOrgChg, changes["target_org_ids_changed"])
			assert.Equal(t, tt.wantTntChg, changes["target_tenant_ids_changed"])
			assert.Equal(t, tt.wantRolesChg, changes["target_roles_changed"])
		})
	}
}

// --- validateTargetingIDs errors (table-driven, covers create + update) ---

func TestAnnouncementService_ValidateTargetingIDs_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		orgCountFn    func(context.Context, []int64) (int, error)
		schoolCountFn func(context.Context, []int64) (int, error)
		orgIDs        []int64
		tenantIDs     []int64
		wantInErr     string
		wantType      interface{} // nil = don't check type
	}{
		{
			name:       "OrgNotFound",
			orgCountFn: func(context.Context, []int64) (int, error) { return 0, nil },
			orgIDs:     []int64{999},
			wantInErr:  "one or more organization IDs do not exist",
			wantType:   &platformSvc.InvalidDataError{},
		},
		{
			name:       "OrgLookupError",
			orgCountFn: func(context.Context, []int64) (int, error) { return 0, fmt.Errorf("db timeout") },
			orgIDs:     []int64{10},
			wantInErr:  "db timeout",
		},
		{
			name:       "InvalidOrgID",
			orgCountFn: func(context.Context, []int64) (int, error) { return 0, organizationtenancy.ErrInvalidOrganization },
			orgIDs:     []int64{0},
			wantInErr:  "invalid organization",
			wantType:   &platformSvc.InvalidDataError{},
		},
		{
			name:          "SchoolNotFound",
			schoolCountFn: func(context.Context, []int64) (int, error) { return 0, nil },
			tenantIDs:     []int64{888},
			wantInErr:     "one or more school (tenant) IDs do not exist",
			wantType:      &platformSvc.InvalidDataError{},
		},
		{
			name:          "SchoolLookupError",
			schoolCountFn: func(context.Context, []int64) (int, error) { return 0, fmt.Errorf("school db error") },
			tenantIDs:     []int64{10},
			wantInErr:     "school db error",
		},
	}

	// Test each error via both CreateAnnouncement and UpdateAnnouncement
	for _, tt := range tests {
		t.Run("Create_"+tt.name, func(t *testing.T) {
			svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
				if tt.orgCountFn != nil {
					m.orgRepo.countByIDsFn = tt.orgCountFn
				}
				if tt.schoolCountFn != nil {
					m.schoolRepo.CountByIDsFn = tt.schoolCountFn
				}
			})
			a := validAnnouncement()
			a.TargetOrgIDs = tt.orgIDs
			a.TargetTenantIDs = tt.tenantIDs

			err := svc.CreateAnnouncement(context.Background(), a, 42, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantInErr)
			if tt.wantType != nil {
				assert.IsType(t, tt.wantType, err)
			}
		})

		t.Run("Update_"+tt.name, func(t *testing.T) {
			svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
				m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
					return existingAnnouncement(), nil
				}
				if tt.orgCountFn != nil {
					m.orgRepo.countByIDsFn = tt.orgCountFn
				}
				if tt.schoolCountFn != nil {
					m.schoolRepo.CountByIDsFn = tt.schoolCountFn
				}
			})
			a := validAnnouncement()
			a.TargetOrgIDs = tt.orgIDs
			a.TargetTenantIDs = tt.tenantIDs
			a.CreatedBy = 1

			err := svc.UpdateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantInErr)
			if tt.wantType != nil {
				assert.IsType(t, tt.wantType, err)
			}
		})
	}
}

// TestAnnouncementService_CreateAnnouncement_RejectsSoftDeletedOrgTarget pins the
// invariant that new announcements cannot target a soft-deleted organization.
// CountByIDs excludes deleted_at IS NOT NULL rows at the repository layer, so a
// soft-deleted org ID looks to the service like a nonexistent ID and surfaces as
// InvalidDataError. This guards against an operator pinning an announcement to a
// trashed Träger — which would silently vanish once the org is restored and could
// leak the announcement to unintended audiences.
func TestAnnouncementService_CreateAnnouncement_RejectsSoftDeletedOrgTarget(t *testing.T) {
	t.Parallel()

	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		// Simulate repository behavior: the org row exists but is soft-deleted,
		// so CountByIDs returns 0 for the requested ID.
		m.orgRepo.countByIDsFn = func(_ context.Context, ids []int64) (int, error) {
			assert.Equal(t, []int64{42}, ids)
			return 0, nil
		}
		m.announcementRepo.createFn = func(context.Context, *platform.Announcement) error {
			t.Fatal("Create should not be called when targeting a soft-deleted org")
			return nil
		}
	})

	a := validAnnouncement()
	a.TargetOrgIDs = []int64{42}

	err := svc.CreateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
	assert.Contains(t, err.Error(), "one or more organization IDs do not exist")
}

// TestAnnouncementService_CreateAnnouncement_RejectsSoftDeletedSchoolTarget is
// the school-tenant analogue of the org test above.
func TestAnnouncementService_CreateAnnouncement_RejectsSoftDeletedSchoolTarget(t *testing.T) {
	t.Parallel()

	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.schoolRepo.CountByIDsFn = func(_ context.Context, ids []int64) (int, error) {
			assert.Equal(t, []int64{99}, ids)
			return 0, nil
		}
		m.announcementRepo.createFn = func(context.Context, *platform.Announcement) error {
			t.Fatal("Create should not be called when targeting a soft-deleted school")
			return nil
		}
	})

	a := validAnnouncement()
	a.TargetTenantIDs = []int64{99}

	err := svc.CreateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
	assert.Contains(t, err.Error(), "one or more school (tenant) IDs do not exist")
}

// TestAnnouncementService_UpdateAnnouncement_PreservesHistoricalDeletedOrgTarget
// is the load-bearing invariant behind validateNewTargetingIDs: an announcement
// whose historical TargetOrgIDs reference an org that has since been soft-deleted
// MUST still round-trip through an unrelated edit (e.g. title change). Empty
// TargetOrgIDs means "global", so silently pruning the deleted ID would widen
// the audience of an already-published scoped announcement. Instead the diff
// helper only validates NEWLY-ADDED targets; historical targets skip CountByIDs
// entirely. Without this test a future refactor could resurrect the old
// validateTargetingIDs call-path on update and break the preservation contract.
func TestAnnouncementService_UpdateAnnouncement_PreservesHistoricalDeletedOrgTarget(t *testing.T) {
	t.Parallel()

	var countByIDsCalls int
	var updateCalled bool
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
			existing := existingAnnouncement()
			existing.TargetOrgIDs = []int64{42}
			existing.CreatedBy = 1
			return existing, nil
		}
		// Simulate org 42 having been soft-deleted since the announcement was
		// created: CountByIDs would return 0. The diff helper must ensure this
		// repo is not queried for unchanged historical IDs.
		m.orgRepo.countByIDsFn = func(_ context.Context, ids []int64) (int, error) {
			countByIDsCalls++
			t.Errorf("CountByIDs must not be called when historical target is unchanged, got ids=%v", ids)
			return 0, nil
		}
		m.announcementRepo.updateFn = func(context.Context, *platform.Announcement) error {
			updateCalled = true
			return nil
		}
	})

	a := validAnnouncement()
	a.ID = 1
	a.Title = "Updated title"
	a.TargetOrgIDs = []int64{42} // unchanged historical target
	a.CreatedBy = 1

	err := svc.UpdateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err, "update should succeed even though org 42 is now soft-deleted")
	assert.True(t, updateCalled, "repo.Update should have been invoked")
	assert.Equal(t, 0, countByIDsCalls, "CountByIDs must not be called for unchanged historical IDs")
}

// TestAnnouncementService_UpdateAnnouncement_PreservesHistoricalDeletedSchoolTarget
// is the school-tenant analogue of the org preservation test above.
func TestAnnouncementService_UpdateAnnouncement_PreservesHistoricalDeletedSchoolTarget(t *testing.T) {
	t.Parallel()

	var countByIDsCalls int
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
			existing := existingAnnouncement()
			existing.TargetTenantIDs = []int64{77}
			existing.CreatedBy = 1
			return existing, nil
		}
		m.schoolRepo.CountByIDsFn = func(_ context.Context, ids []int64) (int, error) {
			countByIDsCalls++
			t.Errorf("school CountByIDs must not be called for unchanged historical IDs, got ids=%v", ids)
			return 0, nil
		}
		m.announcementRepo.updateFn = func(context.Context, *platform.Announcement) error { return nil }
	})

	a := validAnnouncement()
	a.ID = 1
	a.Content = "Updated content"
	a.TargetTenantIDs = []int64{77} // unchanged historical target, now soft-deleted
	a.CreatedBy = 1

	err := svc.UpdateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err, "update should succeed even though school 77 is now soft-deleted")
	assert.Equal(t, 0, countByIDsCalls)
}

// TestAnnouncementService_UpdateAnnouncement_RejectsNewlyAddedSoftDeletedOrgTarget
// complements the preservation tests: operators cannot sneak a soft-deleted org
// in as a NEW target during an update. Only additions relative to the existing
// record are validated, so this path must still flow through CountByIDs.
func TestAnnouncementService_UpdateAnnouncement_RejectsNewlyAddedSoftDeletedOrgTarget(t *testing.T) {
	t.Parallel()

	var updateCalled bool
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
			existing := existingAnnouncement()
			existing.TargetOrgIDs = []int64{42} // legitimate historical target
			existing.CreatedBy = 1
			return existing, nil
		}
		// Only the NEW addition (org 99) should reach the repo; CountByIDs
		// returns 0 to simulate it being soft-deleted.
		m.orgRepo.countByIDsFn = func(_ context.Context, ids []int64) (int, error) {
			assert.Equal(t, []int64{99}, ids, "only the newly-added org ID should be validated")
			return 0, nil
		}
		m.announcementRepo.updateFn = func(context.Context, *platform.Announcement) error {
			updateCalled = true
			return nil
		}
	})

	a := validAnnouncement()
	a.ID = 1
	a.TargetOrgIDs = []int64{42, 99} // 42 historical, 99 newly added + soft-deleted
	a.CreatedBy = 1

	err := svc.UpdateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
	assert.Contains(t, err.Error(), "one or more organization IDs do not exist")
	assert.False(t, updateCalled, "repo.Update must not run when new target is soft-deleted")
}

func TestAnnouncementService_DeleteAnnouncement_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.DeleteAnnouncement(ctx, 999, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
}

func TestAnnouncementService_ListAnnouncements_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		listFn: func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			return []*platform.Announcement{
				{
					Model: base.Model{ID: 1},
					Title: "Ann 1",
				},
				{
					Model: base.Model{ID: 2},
					Title: "Ann 2",
				},
			}, nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcements, err := service.ListAnnouncements(ctx, false)
	require.NoError(t, err)
	assert.Len(t, announcements, 2)
}

func TestAnnouncementService_PublishAnnouncement_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model: base.Model{
					ID: 1,
				},
				Title:    "Test",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
		publishFn: func(ctx context.Context, id int64) error {
			return nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.PublishAnnouncement(ctx, 1, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

func TestAnnouncementService_PublishAnnouncement_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.PublishAnnouncement(ctx, 999, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
}

func TestAnnouncementService_UnpublishAnnouncement_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model: base.Model{
					ID: 1,
				},
				Title:       "Test",
				Content:     "Content",
				Type:        platform.TypeAnnouncement,
				Severity:    platform.SeverityInfo,
				PublishedAt: &now,
			}, nil
		},
		unpublishFn: func(ctx context.Context, id int64) error {
			return nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.UnpublishAnnouncement(ctx, 1, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

func TestAnnouncementService_GetUnreadForUser_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platform.Announcement, error) {
			return []*platform.Announcement{
				{
					Model: base.Model{ID: 1},
					Title: "Unread 1",
				},
				{
					Model: base.Model{ID: 2},
					Title: "Unread 2",
				},
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcements, err := service.GetUnreadForUser(ctx, 1, []string{"admin"}, 10, 5)
	require.NoError(t, err)
	assert.Len(t, announcements, 2)
}

func TestAnnouncementService_CountUnread_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		countUnreadFn: func(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error) {
			return 5, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	count, err := service.CountUnread(ctx, 1, []string{"admin"}, 10, 5)
	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestAnnouncementService_MarkSeen_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		markSeenFn: func(ctx context.Context, userID, announcementID int64) error {
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.MarkSeen(ctx, 1, 1)
	require.NoError(t, err)
}

func TestAnnouncementService_MarkDismissed_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		markDismissedFn: func(ctx context.Context, userID, announcementID int64) error {
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.MarkDismissed(ctx, 1, 1)
	require.NoError(t, err)
}

func TestAnnouncementService_GetStats_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model: base.Model{
					ID: 42,
				},
				Title:    "Test",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{
		getStatsFn: func(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error) {
			return &platform.AnnouncementStats{
				AnnouncementID: announcementID,
				TargetCount:    100,
				SeenCount:      50,
				DismissedCount: 10,
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	stats, err := service.GetStats(ctx, 42)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(42), stats.AnnouncementID)
	assert.Equal(t, 100, stats.TargetCount)
	assert.Equal(t, 50, stats.SeenCount)
}

func TestAnnouncementService_GetStats_AnnouncementNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.GetStats(ctx, 999)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
}

func TestAnnouncementService_GetViewDetails_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model: base.Model{
					ID: 42,
				},
				Title:    "Test",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{
		getViewDetailsFn: func(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error) {
			return []*platform.AnnouncementViewDetail{
				{UserID: 42, UserName: "User 1", SeenAt: time.Now(), Dismissed: false},
				{UserID: 43, UserName: "User 2", SeenAt: time.Now(), Dismissed: true},
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	details, err := service.GetViewDetails(ctx, 42)
	require.NoError(t, err)
	assert.Len(t, details, 2)
	assert.Equal(t, int64(42), details[0].UserID)
	assert.Equal(t, "User 1", details[0].UserName)
}

func TestAnnouncementService_GetViewDetails_AnnouncementNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.GetViewDetails(ctx, 999)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
}

func TestIsAnnouncementExpired_Nil(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	a := &platform.Announcement{}
	assert.False(t, platformSvc.IsAnnouncementExpired(a, now))
}

func TestIsAnnouncementExpired_Future(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	a := &platform.Announcement{ExpiresAt: &future}
	assert.False(t, platformSvc.IsAnnouncementExpired(a, now))
}

func TestIsAnnouncementExpired_Past(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	a := &platform.Announcement{ExpiresAt: &past}
	assert.True(t, platformSvc.IsAnnouncementExpired(a, now))
}
