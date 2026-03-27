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

func TestAnnouncementService_CreateAnnouncement_Success(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		createFn: func(ctx context.Context, announcement *platform.Announcement) error {
			announcement.ID = 42
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

	announcement := &platform.Announcement{
		Title:    "Test Announcement",
		Content:  "Test content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}

	err := service.CreateAnnouncement(ctx, announcement, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	assert.Equal(t, int64(42), announcement.CreatedBy)
}

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

func TestAnnouncementService_CreateAnnouncement_RepositoryError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		createFn: func(ctx context.Context, announcement *platform.Announcement) error {
			return fmt.Errorf("database error")
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

	announcement := &platform.Announcement{
		Title:    "Test",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}

	err := service.CreateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestAnnouncementService_GetAnnouncement_Success(t *testing.T) {
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
	viewRepo := &mockAnnouncementViewRepoShared{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcement, err := service.GetAnnouncement(ctx, 42)
	require.NoError(t, err)
	assert.NotNil(t, announcement)
	assert.Equal(t, int64(42), announcement.ID)
}

func TestAnnouncementService_GetAnnouncement_NotFound(t *testing.T) {
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

	_, err := service.GetAnnouncement(ctx, 999)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
}

func TestAnnouncementService_UpdateAnnouncement_Success(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model: base.Model{
					ID: 1,
				},
				Title:    "Old Title",
				Content:  "Old Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
		updateFn: func(ctx context.Context, announcement *platform.Announcement) error {
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

	announcement := &platform.Announcement{
		Title:    "New Title",
		Content:  "New Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}
	announcement.CreatedBy = 1

	err := service.UpdateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

func TestAnnouncementService_UpdateAnnouncement_NilAnnouncement(t *testing.T) {
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

	err := service.UpdateAnnouncement(ctx, nil, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestAnnouncementService_UpdateAnnouncement_NotFound(t *testing.T) {
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

	announcement := &platform.Announcement{
		Model: base.Model{
			ID: 999,
		},
		Title:    "Test",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}

	err := service.UpdateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
}

func TestAnnouncementService_DeleteAnnouncement_Success(t *testing.T) {
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
		deleteFn: func(ctx context.Context, id int64) error {
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

	err := service.DeleteAnnouncement(ctx, 1, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
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
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string, orgID int64, tenantID int64) ([]*platform.Announcement, error) {
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

	announcements, err := service.GetUnreadForUser(ctx, 1, []string{"admin"}, 0, 0)
	require.NoError(t, err)
	assert.Len(t, announcements, 2)
}

func TestAnnouncementService_CountUnread_Success(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		countUnreadFn: func(ctx context.Context, userID int64, userRoles []string, orgID int64, tenantID int64) (int, error) {
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

	count, err := service.CountUnread(ctx, 1, []string{"admin"}, 0, 0)
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

// --- Org/Tenant passthrough for GetUnreadForUser and CountUnread ---

func TestAnnouncementService_GetUnreadForUser_OrgTenantPassthrough(t *testing.T) {
	tests := []struct {
		name     string
		orgID    int64
		tenantID int64
	}{
		{"passes org and tenant", 5, 10},
		{"zero values", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedOrgID, capturedTenantID int64
			svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
				m.viewRepo.getUnreadForUserFn = func(_ context.Context, _ int64, _ []string, orgID, tenantID int64) ([]*platform.Announcement, error) {
					capturedOrgID = orgID
					capturedTenantID = tenantID
					return []*platform.Announcement{}, nil
				}
			})
			_, err := svc.GetUnreadForUser(context.Background(), 42, []string{"teacher"}, tt.orgID, tt.tenantID)
			require.NoError(t, err)
			assert.Equal(t, tt.orgID, capturedOrgID)
			assert.Equal(t, tt.tenantID, capturedTenantID)
		})
	}
}

func TestAnnouncementService_CountUnread_OrgTenantPassthrough(t *testing.T) {
	var capturedOrgID, capturedTenantID int64
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.viewRepo.countUnreadFn = func(_ context.Context, _ int64, _ []string, orgID, tenantID int64) (int, error) {
			capturedOrgID = orgID
			capturedTenantID = tenantID
			return 3, nil
		}
	})
	count, err := svc.CountUnread(context.Background(), 42, []string{"teacher"}, 5, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, int64(5), capturedOrgID)
	assert.Equal(t, int64(10), capturedTenantID)
}

// --- FindByID error propagation (table-driven) ---

func TestAnnouncementService_FindByIDError(t *testing.T) {
	findByIDErr := func(m *testAnnouncementMocks) {
		m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
			return nil, fmt.Errorf("db error")
		}
	}
	tests := []struct {
		name string
		call func(svc platformSvc.AnnouncementService) error
	}{
		{"GetAnnouncement", func(svc platformSvc.AnnouncementService) error {
			_, err := svc.GetAnnouncement(context.Background(), 42)
			return err
		}},
		{"GetStats", func(svc platformSvc.AnnouncementService) error {
			_, err := svc.GetStats(context.Background(), 42)
			return err
		}},
		{"GetViewDetails", func(svc platformSvc.AnnouncementService) error {
			_, err := svc.GetViewDetails(context.Background(), 42)
			return err
		}},
		{"UpdateAnnouncement", func(svc platformSvc.AnnouncementService) error {
			a := validAnnouncement()
			a.ID = 99
			a.CreatedBy = 1
			return svc.UpdateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
		}},
		{"DeleteAnnouncement", func(svc platformSvc.AnnouncementService) error {
			return svc.DeleteAnnouncement(context.Background(), 1, 1, net.ParseIP("127.0.0.1"))
		}},
		{"PublishAnnouncement", func(svc platformSvc.AnnouncementService) error {
			return svc.PublishAnnouncement(context.Background(), 1, 1, net.ParseIP("127.0.0.1"))
		}},
		{"UnpublishAnnouncement", func(svc platformSvc.AnnouncementService) error {
			return svc.UnpublishAnnouncement(context.Background(), 1, 1, net.ParseIP("127.0.0.1"))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestAnnouncementService(findByIDErr)
			err := tt.call(svc)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "db error")
		})
	}
}

// --- Repository error passthrough (table-driven) ---

func TestAnnouncementService_RepositoryErrors(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(m *testAnnouncementMocks)
		call      func(svc platformSvc.AnnouncementService) error
		wantInErr string
	}{
		{
			name: "GetUnreadForUser",
			setup: func(m *testAnnouncementMocks) {
				m.viewRepo.getUnreadForUserFn = func(context.Context, int64, []string, int64, int64) ([]*platform.Announcement, error) {
					return nil, fmt.Errorf("database connection lost")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				_, err := svc.GetUnreadForUser(context.Background(), 42, []string{"teacher"}, 5, 10)
				return err
			},
			wantInErr: "database connection lost",
		},
		{
			name: "CountUnread",
			setup: func(m *testAnnouncementMocks) {
				m.viewRepo.countUnreadFn = func(context.Context, int64, []string, int64, int64) (int, error) {
					return 0, fmt.Errorf("database timeout")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				_, err := svc.CountUnread(context.Background(), 42, []string{"teacher"}, 5, 10)
				return err
			},
			wantInErr: "database timeout",
		},
		{
			name: "GetStats",
			setup: func(m *testAnnouncementMocks) {
				m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
					return existingAnnouncement(), nil
				}
				m.viewRepo.getStatsFn = func(context.Context, int64) (*platform.AnnouncementStats, error) {
					return nil, fmt.Errorf("stats query failed")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				_, err := svc.GetStats(context.Background(), 42)
				return err
			},
			wantInErr: "stats query failed",
		},
		{
			name: "GetViewDetails",
			setup: func(m *testAnnouncementMocks) {
				m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
					return existingAnnouncement(), nil
				}
				m.viewRepo.getViewDetailsFn = func(context.Context, int64) ([]*platform.AnnouncementViewDetail, error) {
					return nil, fmt.Errorf("view details query failed")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				_, err := svc.GetViewDetails(context.Background(), 42)
				return err
			},
			wantInErr: "view details query failed",
		},
		{
			name: "UpdateAnnouncement",
			setup: func(m *testAnnouncementMocks) {
				m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
					return existingAnnouncement(), nil
				}
				m.announcementRepo.updateFn = func(context.Context, *platform.Announcement) error {
					return fmt.Errorf("update failed")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				a := validAnnouncement()
				a.CreatedBy = 1
				return svc.UpdateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
			},
			wantInErr: "update failed",
		},
		{
			name: "DeleteAnnouncement",
			setup: func(m *testAnnouncementMocks) {
				m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
					return existingAnnouncement(), nil
				}
				m.announcementRepo.deleteFn = func(context.Context, int64) error {
					return fmt.Errorf("delete failed")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				return svc.DeleteAnnouncement(context.Background(), 1, 1, net.ParseIP("127.0.0.1"))
			},
			wantInErr: "delete failed",
		},
		{
			name: "PublishAnnouncement",
			setup: func(m *testAnnouncementMocks) {
				m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
					return existingAnnouncement(), nil
				}
				m.announcementRepo.publishFn = func(context.Context, int64) error {
					return fmt.Errorf("publish failed")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				return svc.PublishAnnouncement(context.Background(), 1, 1, net.ParseIP("127.0.0.1"))
			},
			wantInErr: "publish failed",
		},
		{
			name: "UnpublishAnnouncement",
			setup: func(m *testAnnouncementMocks) {
				now := time.Now()
				m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
					a := existingAnnouncement()
					a.PublishedAt = &now
					return a, nil
				}
				m.announcementRepo.unpublishFn = func(context.Context, int64) error {
					return fmt.Errorf("unpublish failed")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				return svc.UnpublishAnnouncement(context.Background(), 1, 1, net.ParseIP("127.0.0.1"))
			},
			wantInErr: "unpublish failed",
		},
		{
			name: "ListAnnouncements",
			setup: func(m *testAnnouncementMocks) {
				m.announcementRepo.listFn = func(context.Context, bool) ([]*platform.Announcement, error) {
					return nil, fmt.Errorf("list failed")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				_, err := svc.ListAnnouncements(context.Background(), false)
				return err
			},
			wantInErr: "list failed",
		},
		{
			name: "MarkSeen",
			setup: func(m *testAnnouncementMocks) {
				m.viewRepo.markSeenFn = func(context.Context, int64, int64) error {
					return fmt.Errorf("mark seen failed")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				return svc.MarkSeen(context.Background(), 1, 1)
			},
			wantInErr: "mark seen failed",
		},
		{
			name: "MarkDismissed",
			setup: func(m *testAnnouncementMocks) {
				m.viewRepo.markDismissedFn = func(context.Context, int64, int64) error {
					return fmt.Errorf("mark dismissed failed")
				}
			},
			call: func(svc platformSvc.AnnouncementService) error {
				return svc.MarkDismissed(context.Background(), 1, 1)
			},
			wantInErr: "mark dismissed failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestAnnouncementService(tt.setup)
			err := tt.call(svc)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantInErr)
		})
	}
}

// --- UnpublishAnnouncement NotFound ---

func TestAnnouncementService_UnpublishAnnouncement_NotFound(t *testing.T) {
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
			return nil, nil
		}
	})
	err := svc.UnpublishAnnouncement(context.Background(), 999, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
}

// --- Targeting: create with targeting, nil normalization ---

func TestAnnouncementService_CreateAnnouncement_WithTargeting(t *testing.T) {
	var captured *platform.Announcement
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.createFn = func(_ context.Context, a *platform.Announcement) error {
			captured = a
			a.ID = 99
			return nil
		}
	})

	a := validAnnouncement()
	a.TargetOrgIDs = []int64{5, 10}
	a.TargetTenantIDs = []int64{12}

	err := svc.CreateAnnouncement(context.Background(), a, 42, nil)
	require.NoError(t, err)
	assert.Equal(t, []int64{5, 10}, captured.TargetOrgIDs)
	assert.Equal(t, []int64{12}, captured.TargetTenantIDs)
}

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

func TestAnnouncementService_UpdateAnnouncement_WithTargeting(t *testing.T) {
	var captured *platform.Announcement
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
			return existingAnnouncement(), nil
		}
		m.announcementRepo.updateFn = func(_ context.Context, a *platform.Announcement) error {
			captured = a
			return nil
		}
	})

	a := validAnnouncement()
	a.TargetOrgIDs = []int64{3, 4}
	a.TargetTenantIDs = []int64{7}
	a.CreatedBy = 1

	err := svc.UpdateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 4}, captured.TargetOrgIDs)
	assert.Equal(t, []int64{7}, captured.TargetTenantIDs)
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

// --- ListAnnouncements includeInactive flag ---

func TestAnnouncementService_ListAnnouncements_IncludeInactive(t *testing.T) {
	var capturedFlag bool
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.listFn = func(_ context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			capturedFlag = includeInactive
			return []*platform.Announcement{}, nil
		}
	})
	_, err := svc.ListAnnouncements(context.Background(), true)
	require.NoError(t, err)
	assert.True(t, capturedFlag)
}

// --- Nil logger and audit log error resilience ---

func TestAnnouncementService_GetLogger_NilLogger(t *testing.T) {
	svc := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo: &mockAnnouncementRepoShared{
			createFn: func(_ context.Context, a *platform.Announcement) error { a.ID = 1; return nil },
		},
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo: &mockAuditLogRepoShared{
			createFn: func(context.Context, *platform.OperatorAuditLog) error { return fmt.Errorf("audit log error") },
		},
		DB:     &bun.DB{},
		Logger: nil,
	})

	err := svc.CreateAnnouncement(context.Background(), validAnnouncement(), 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

func TestAnnouncementService_LogAction_AuditLogSetChangesError(t *testing.T) {
	svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
		m.announcementRepo.findByIDFn = func(context.Context, int64) (*platform.Announcement, error) {
			return existingAnnouncement(), nil
		}
		m.announcementRepo.updateFn = func(context.Context, *platform.Announcement) error { return nil }
		m.auditLogRepo.createFn = func(context.Context, *platform.OperatorAuditLog) error {
			return fmt.Errorf("audit create failed")
		}
	})

	a := validAnnouncement()
	a.Title = "New Title"
	a.Content = "New Content"
	a.CreatedBy = 1
	err := svc.UpdateAnnouncement(context.Background(), a, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

// --- validateTargetingIDs errors (table-driven, covers create + update) ---

func TestAnnouncementService_ValidateTargetingIDs_Errors(t *testing.T) {
	tests := []struct {
		name      string
		orgFn     func(context.Context, int64) (*platform.Organization, error)
		schoolFn  func(context.Context, int64) (*platform.School, error)
		orgIDs    []int64
		tenantIDs []int64
		wantInErr string
		wantType  interface{} // nil = don't check type
	}{
		{
			name:      "OrgNotFound",
			orgFn:     func(context.Context, int64) (*platform.Organization, error) { return nil, nil },
			orgIDs:    []int64{999},
			wantInErr: "organization with ID 999 does not exist",
			wantType:  &platformSvc.InvalidDataError{},
		},
		{
			name:      "OrgLookupError",
			orgFn:     func(context.Context, int64) (*platform.Organization, error) { return nil, fmt.Errorf("db timeout") },
			orgIDs:    []int64{10},
			wantInErr: "db timeout",
		},
		{
			name:      "SchoolNotFound",
			schoolFn:  func(context.Context, int64) (*platform.School, error) { return nil, nil },
			tenantIDs: []int64{888},
			wantInErr: "school (tenant) with ID 888 does not exist",
			wantType:  &platformSvc.InvalidDataError{},
		},
		{
			name:      "SchoolLookupError",
			schoolFn:  func(context.Context, int64) (*platform.School, error) { return nil, fmt.Errorf("school db error") },
			tenantIDs: []int64{10},
			wantInErr: "school db error",
		},
	}

	// Test each error via both CreateAnnouncement and UpdateAnnouncement
	for _, tt := range tests {
		t.Run("Create_"+tt.name, func(t *testing.T) {
			svc := newTestAnnouncementService(func(m *testAnnouncementMocks) {
				if tt.orgFn != nil {
					m.orgRepo.findByIDFn = tt.orgFn
				}
				if tt.schoolFn != nil {
					m.schoolRepo.findByIDFn = tt.schoolFn
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
				if tt.orgFn != nil {
					m.orgRepo.findByIDFn = tt.orgFn
				}
				if tt.schoolFn != nil {
					m.schoolRepo.findByIDFn = tt.schoolFn
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
