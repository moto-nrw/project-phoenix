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

func TestAnnouncementService_GetUnreadForUser_PassesOrgAndTenantToRepo(t *testing.T) {
	ctx := context.Background()
	var capturedOrgID, capturedTenantID int64
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string, orgID int64, tenantID int64) ([]*platform.Announcement, error) {
			capturedOrgID = orgID
			capturedTenantID = tenantID
			return []*platform.Announcement{
				{
					Model: base.Model{ID: 10},
					Title: "Targeted Announcement",
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

	announcements, err := service.GetUnreadForUser(ctx, 42, []string{"teacher"}, 5, 10)
	require.NoError(t, err)
	assert.Len(t, announcements, 1)
	assert.Equal(t, int64(5), capturedOrgID, "orgID should be passed through to the repository")
	assert.Equal(t, int64(10), capturedTenantID, "tenantID should be passed through to the repository")
}

func TestAnnouncementService_CountUnread_PassesOrgAndTenantToRepo(t *testing.T) {
	ctx := context.Background()
	var capturedOrgID, capturedTenantID int64
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		countUnreadFn: func(ctx context.Context, userID int64, userRoles []string, orgID int64, tenantID int64) (int, error) {
			capturedOrgID = orgID
			capturedTenantID = tenantID
			return 3, nil
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

	count, err := service.CountUnread(ctx, 42, []string{"teacher"}, 5, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, int64(5), capturedOrgID, "orgID should be passed through to the repository")
	assert.Equal(t, int64(10), capturedTenantID, "tenantID should be passed through to the repository")
}

func TestAnnouncementService_GetUnreadForUser_RepositoryError(t *testing.T) {
	ctx := context.Background()
	viewRepo := &mockAnnouncementViewRepoShared{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string, orgID int64, tenantID int64) ([]*platform.Announcement, error) {
			return nil, fmt.Errorf("database connection lost")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     &mockAnnouncementRepoShared{},
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.GetUnreadForUser(ctx, 42, []string{"teacher"}, 5, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database connection lost")
}

func TestAnnouncementService_CountUnread_RepositoryError(t *testing.T) {
	ctx := context.Background()
	viewRepo := &mockAnnouncementViewRepoShared{
		countUnreadFn: func(ctx context.Context, userID int64, userRoles []string, orgID int64, tenantID int64) (int, error) {
			return 0, fmt.Errorf("database timeout")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     &mockAnnouncementRepoShared{},
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.CountUnread(ctx, 42, []string{"teacher"}, 5, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database timeout")
}

func TestAnnouncementService_GetUnreadForUser_ZeroOrgAndTenant(t *testing.T) {
	ctx := context.Background()
	var capturedOrgID, capturedTenantID int64
	viewRepo := &mockAnnouncementViewRepoShared{
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRoles []string, orgID int64, tenantID int64) ([]*platform.Announcement, error) {
			capturedOrgID = orgID
			capturedTenantID = tenantID
			return []*platform.Announcement{}, nil
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     &mockAnnouncementRepoShared{},
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcements, err := service.GetUnreadForUser(ctx, 42, []string{"admin"}, 0, 0)
	require.NoError(t, err)
	assert.Empty(t, announcements)
	assert.Equal(t, int64(0), capturedOrgID)
	assert.Equal(t, int64(0), capturedTenantID)
}

func TestAnnouncementService_GetStats_RepositoryError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Title:    "Test",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{
		getStatsFn: func(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error) {
			return nil, fmt.Errorf("stats query failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.GetStats(ctx, 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stats query failed")
}

func TestAnnouncementService_GetViewDetails_RepositoryError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Title:    "Test",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
	}
	viewRepo := &mockAnnouncementViewRepoShared{
		getViewDetailsFn: func(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error) {
			return nil, fmt.Errorf("view details query failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.GetViewDetails(ctx, 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "view details query failed")
}

func TestAnnouncementService_CreateAnnouncement_WithTargeting(t *testing.T) {
	ctx := context.Background()
	var capturedAnnouncement *platform.Announcement
	announcementRepo := &mockAnnouncementRepoShared{
		createFn: func(ctx context.Context, announcement *platform.Announcement) error {
			capturedAnnouncement = announcement
			announcement.ID = 99
			return nil
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcement := &platform.Announcement{
		Title:           "Targeted",
		Content:         "Content",
		Type:            platform.TypeAnnouncement,
		Severity:        platform.SeverityInfo,
		TargetOrgIDs:    []int64{5, 10},
		TargetTenantIDs: []int64{12},
	}

	err := service.CreateAnnouncement(ctx, announcement, 42, nil)
	require.NoError(t, err)
	assert.Equal(t, []int64{5, 10}, capturedAnnouncement.TargetOrgIDs)
	assert.Equal(t, []int64{12}, capturedAnnouncement.TargetTenantIDs)
}

func TestAnnouncementService_CreateAnnouncement_NilTargetingNormalized(t *testing.T) {
	ctx := context.Background()
	var capturedAnnouncement *platform.Announcement
	announcementRepo := &mockAnnouncementRepoShared{
		createFn: func(ctx context.Context, announcement *platform.Announcement) error {
			capturedAnnouncement = announcement
			announcement.ID = 100
			return nil
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcement := &platform.Announcement{
		Title:           "Global",
		Content:         "Content",
		Type:            platform.TypeAnnouncement,
		Severity:        platform.SeverityInfo,
		TargetOrgIDs:    nil,
		TargetTenantIDs: nil,
	}

	err := service.CreateAnnouncement(ctx, announcement, 42, nil)
	require.NoError(t, err)
	assert.NotNil(t, capturedAnnouncement.TargetOrgIDs, "nil TargetOrgIDs should be normalized to empty slice by Validate")
	assert.NotNil(t, capturedAnnouncement.TargetTenantIDs, "nil TargetTenantIDs should be normalized to empty slice by Validate")
	assert.Empty(t, capturedAnnouncement.TargetOrgIDs)
	assert.Empty(t, capturedAnnouncement.TargetTenantIDs)
}

func TestAnnouncementService_UpdateAnnouncement_RepositoryError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model:    base.Model{ID: 1},
				Title:    "Test",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
		updateFn: func(ctx context.Context, announcement *platform.Announcement) error {
			return fmt.Errorf("update failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcement := &platform.Announcement{
		Title:    "Updated",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}
	announcement.CreatedBy = 1

	err := service.UpdateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update failed")
}

func TestAnnouncementService_UpdateAnnouncement_FindByIDError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, fmt.Errorf("db connection lost")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcement := &platform.Announcement{
		Model:    base.Model{ID: 99},
		Title:    "Test",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}
	announcement.CreatedBy = 1

	err := service.UpdateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db connection lost")
}

func TestAnnouncementService_UpdateAnnouncement_ValidationError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model:    base.Model{ID: 1},
				Title:    "Old",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	// Empty title should fail validation
	announcement := &platform.Announcement{
		Title:    "",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}
	announcement.CreatedBy = 1

	err := service.UpdateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestAnnouncementService_UpdateAnnouncement_WithTargeting(t *testing.T) {
	ctx := context.Background()
	var capturedAnnouncement *platform.Announcement
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model:    base.Model{ID: 1},
				Title:    "Old",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
		updateFn: func(ctx context.Context, announcement *platform.Announcement) error {
			capturedAnnouncement = announcement
			return nil
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	announcement := &platform.Announcement{
		Title:           "Updated",
		Content:         "Content",
		Type:            platform.TypeAnnouncement,
		Severity:        platform.SeverityInfo,
		TargetOrgIDs:    []int64{3, 4},
		TargetTenantIDs: []int64{7},
	}
	announcement.CreatedBy = 1

	err := service.UpdateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 4}, capturedAnnouncement.TargetOrgIDs)
	assert.Equal(t, []int64{7}, capturedAnnouncement.TargetTenantIDs)
}

func TestAnnouncementService_UpdateAnnouncement_AuditLogTracksTargetingChanges(t *testing.T) {
	ctx := context.Background()
	var capturedAuditEntry *platform.OperatorAuditLog
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model:           base.Model{ID: 1},
				Title:           "Title",
				Content:         "Content",
				Type:            platform.TypeAnnouncement,
				Severity:        platform.SeverityInfo,
				TargetRoles:     []string{"admin"},
				TargetOrgIDs:    []int64{1},
				TargetTenantIDs: []int64{10},
				CreatedBy:       1,
			}, nil
		},
		updateFn: func(ctx context.Context, announcement *platform.Announcement) error {
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			capturedAuditEntry = entry
			return nil
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	// Change all targeting fields
	announcement := &platform.Announcement{
		Title:           "Title",
		Content:         "Content",
		Type:            platform.TypeAnnouncement,
		Severity:        platform.SeverityInfo,
		TargetRoles:     []string{"admin", "user"},
		TargetOrgIDs:    []int64{1, 2},
		TargetTenantIDs: []int64{20},
		CreatedBy:       1,
	}

	err := service.UpdateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotNil(t, capturedAuditEntry, "audit log entry should be created")

	changes, err := capturedAuditEntry.GetChanges()
	require.NoError(t, err)

	assert.Equal(t, true, changes["target_org_ids_changed"], "org IDs changed from [1] to [1,2]")
	assert.Equal(t, true, changes["target_tenant_ids_changed"], "tenant IDs changed from [10] to [20]")
	assert.Equal(t, true, changes["target_roles_changed"], "roles changed from [admin] to [admin,user]")
	assert.Equal(t, false, changes["title_changed"], "title unchanged")
}

func TestAnnouncementService_UpdateAnnouncement_AuditLogTracksNoTargetingChanges(t *testing.T) {
	ctx := context.Background()
	var capturedAuditEntry *platform.OperatorAuditLog
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model:           base.Model{ID: 1},
				Title:           "Title",
				Content:         "Content",
				Type:            platform.TypeAnnouncement,
				Severity:        platform.SeverityInfo,
				TargetRoles:     []string{"admin"},
				TargetOrgIDs:    []int64{5},
				TargetTenantIDs: []int64{},
				CreatedBy:       1,
			}, nil
		},
		updateFn: func(ctx context.Context, announcement *platform.Announcement) error {
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			capturedAuditEntry = entry
			return nil
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	// Same targeting as existing
	announcement := &platform.Announcement{
		Title:           "Title",
		Content:         "Content",
		Type:            platform.TypeAnnouncement,
		Severity:        platform.SeverityInfo,
		TargetRoles:     []string{"admin"},
		TargetOrgIDs:    []int64{5},
		TargetTenantIDs: []int64{},
		CreatedBy:       1,
	}

	err := service.UpdateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotNil(t, capturedAuditEntry)

	changes, err := capturedAuditEntry.GetChanges()
	require.NoError(t, err)

	assert.Equal(t, false, changes["target_org_ids_changed"], "org IDs unchanged")
	assert.Equal(t, false, changes["target_tenant_ids_changed"], "tenant IDs unchanged")
	assert.Equal(t, false, changes["target_roles_changed"], "roles unchanged")
}

func TestAnnouncementService_DeleteAnnouncement_FindByIDError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.DeleteAnnouncement(ctx, 1, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestAnnouncementService_DeleteAnnouncement_RepositoryError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model:    base.Model{ID: 1},
				Title:    "Test",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
		deleteFn: func(ctx context.Context, id int64) error {
			return fmt.Errorf("delete failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.DeleteAnnouncement(ctx, 1, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete failed")
}

func TestAnnouncementService_PublishAnnouncement_FindByIDError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.PublishAnnouncement(ctx, 1, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestAnnouncementService_PublishAnnouncement_RepositoryError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model:    base.Model{ID: 1},
				Title:    "Test",
				Content:  "Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
		publishFn: func(ctx context.Context, id int64) error {
			return fmt.Errorf("publish failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.PublishAnnouncement(ctx, 1, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish failed")
}

func TestAnnouncementService_UnpublishAnnouncement_NotFound(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, nil
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.UnpublishAnnouncement(ctx, 999, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
}

func TestAnnouncementService_UnpublishAnnouncement_FindByIDError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.UnpublishAnnouncement(ctx, 1, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestAnnouncementService_UnpublishAnnouncement_RepositoryError(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model:       base.Model{ID: 1},
				Title:       "Test",
				Content:     "Content",
				Type:        platform.TypeAnnouncement,
				Severity:    platform.SeverityInfo,
				PublishedAt: &now,
			}, nil
		},
		unpublishFn: func(ctx context.Context, id int64) error {
			return fmt.Errorf("unpublish failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.UnpublishAnnouncement(ctx, 1, 1, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unpublish failed")
}

func TestAnnouncementService_ListAnnouncements_RepositoryError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		listFn: func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			return nil, fmt.Errorf("list failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.ListAnnouncements(ctx, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list failed")
}

func TestAnnouncementService_MarkSeen_RepositoryError(t *testing.T) {
	ctx := context.Background()
	viewRepo := &mockAnnouncementViewRepoShared{
		markSeenFn: func(ctx context.Context, userID, announcementID int64) error {
			return fmt.Errorf("mark seen failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     &mockAnnouncementRepoShared{},
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.MarkSeen(ctx, 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mark seen failed")
}

func TestAnnouncementService_MarkDismissed_RepositoryError(t *testing.T) {
	ctx := context.Background()
	viewRepo := &mockAnnouncementViewRepoShared{
		markDismissedFn: func(ctx context.Context, userID, announcementID int64) error {
			return fmt.Errorf("mark dismissed failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     &mockAnnouncementRepoShared{},
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	err := service.MarkDismissed(ctx, 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mark dismissed failed")
}

func TestAnnouncementService_GetStats_FindByIDError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.GetStats(ctx, 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestAnnouncementService_GetViewDetails_FindByIDError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.GetViewDetails(ctx, 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestAnnouncementService_GetAnnouncement_FindByIDError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return nil, fmt.Errorf("db connection error")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.GetAnnouncement(ctx, 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db connection error")
}

func TestAnnouncementService_GetLogger_NilLogger(t *testing.T) {
	ctx := context.Background()
	// Test with nil logger - service should fallback to slog.Default()
	announcementRepo := &mockAnnouncementRepoShared{
		createFn: func(ctx context.Context, announcement *platform.Announcement) error {
			announcement.ID = 1
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			return fmt.Errorf("audit log error")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         auditLogRepo,
		DB:                   &bun.DB{},
		Logger:               nil, // nil logger
	})

	announcement := &platform.Announcement{
		Title:    "Test",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}

	// Should not panic even with nil logger and audit log error
	err := service.CreateAnnouncement(ctx, announcement, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

func TestAnnouncementService_LogAction_AuditLogSetChangesError(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Announcement, error) {
			return &platform.Announcement{
				Model:    base.Model{ID: 1},
				Title:    "Old",
				Content:  "Old Content",
				Type:     platform.TypeAnnouncement,
				Severity: platform.SeverityInfo,
			}, nil
		},
		updateFn: func(ctx context.Context, announcement *platform.Announcement) error {
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			return fmt.Errorf("audit create failed")
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
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

	// Should succeed even if audit logging fails
	err := service.UpdateAnnouncement(ctx, announcement, 1, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

func TestAnnouncementService_ListAnnouncements_IncludeInactive(t *testing.T) {
	ctx := context.Background()
	var capturedIncludeInactive bool
	announcementRepo := &mockAnnouncementRepoShared{
		listFn: func(ctx context.Context, includeInactive bool) ([]*platform.Announcement, error) {
			capturedIncludeInactive = includeInactive
			return []*platform.Announcement{}, nil
		},
	}

	service := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: &mockAnnouncementViewRepoShared{},
		AuditLogRepo:         &mockAuditLogRepoShared{},
		DB:                   &bun.DB{},
		Logger:               slog.Default(),
	})

	_, err := service.ListAnnouncements(ctx, true)
	require.NoError(t, err)
	assert.True(t, capturedIncludeInactive)
}
