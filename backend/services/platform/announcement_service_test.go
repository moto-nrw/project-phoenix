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
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestAnnouncementService_CreateAnnouncement_NilAnnouncement(t *testing.T) {
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
	schoolRepo       *mockSchoolRepoShared
}

func newTestAnnouncementService(overrides func(m *testAnnouncementMocks)) platformSvc.AnnouncementService {
	m := &testAnnouncementMocks{
		announcementRepo: &mockAnnouncementRepoShared{},
		viewRepo:         &mockAnnouncementViewRepoShared{},
		auditLogRepo:     &mockAuditLogRepoShared{},
		orgRepo:          &mockOrgRepoShared{},
		schoolRepo:       &mockSchoolRepoShared{},
	}
	if overrides != nil {
		overrides(m)
	}
	return platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     m.announcementRepo,
		AnnouncementViewRepo: m.viewRepo,
		AuditLogRepo:         m.auditLogRepo,
		OrgRepo:              m.orgRepo,
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
					m.schoolRepo.countByIDsFn = tt.schoolCountFn
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
					m.schoolRepo.countByIDsFn = tt.schoolCountFn
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

func TestAnnouncementService_DeleteAnnouncement_NotFound(t *testing.T) {
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
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string) ([]*platform.Announcement, error) {
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
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		countUnreadFn: func(ctx context.Context, userID int64, userRoles []string) (int, error) {
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
