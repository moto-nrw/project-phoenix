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
		getUnreadForUserFn: func(ctx context.Context, userID int64, userRole string) ([]*platform.Announcement, error) {
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

	announcements, err := service.GetUnreadForUser(ctx, 1, "admin")
	require.NoError(t, err)
	assert.Len(t, announcements, 2)
}

func TestAnnouncementService_CountUnread_Success(t *testing.T) {
	ctx := context.Background()
	announcementRepo := &mockAnnouncementRepoShared{}
	viewRepo := &mockAnnouncementViewRepoShared{
		countUnreadFn: func(ctx context.Context, userID int64, userRole string) (int, error) {
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

	count, err := service.CountUnread(ctx, 1, "admin")
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
