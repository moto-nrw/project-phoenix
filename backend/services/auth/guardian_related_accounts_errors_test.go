package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

// --- minimal repo mocks (embed the interface; override only what the flow calls) ---

type mockInvitationRepo struct {
	authModels.GuardianInvitationRepository
	findByID              func(int64) (*authModels.GuardianInvitation, error)
	update                func(*authModels.GuardianInvitation) error
	create                func(*authModels.GuardianInvitation) error
	findPendingApprv      func() ([]*authModels.GuardianInvitation, error)
	findByGuardianProfile func(int64) ([]*authModels.GuardianInvitation, error)
}

func (m mockInvitationRepo) FindByID(_ context.Context, id int64) (*authModels.GuardianInvitation, error) {
	return m.findByID(id)
}
func (m mockInvitationRepo) Update(_ context.Context, inv *authModels.GuardianInvitation) error {
	return m.update(inv)
}
func (m mockInvitationRepo) Create(_ context.Context, inv *authModels.GuardianInvitation) error {
	return m.create(inv)
}
func (m mockInvitationRepo) FindPendingApproval(_ context.Context) ([]*authModels.GuardianInvitation, error) {
	return m.findPendingApprv()
}
func (m mockInvitationRepo) FindByGuardianProfileID(_ context.Context, id int64) ([]*authModels.GuardianInvitation, error) {
	if m.findByGuardianProfile == nil {
		return nil, nil
	}
	return m.findByGuardianProfile(id)
}

type mockProfileRepo struct {
	userModels.GuardianProfileRepository
	findByEmail func(string) (*userModels.GuardianProfile, error)
	findByID    func(int64) (*userModels.GuardianProfile, error)
	create      func(*userModels.GuardianProfile) error
	del         func(int64) error
}

func (m mockProfileRepo) FindByEmail(_ context.Context, e string) (*userModels.GuardianProfile, error) {
	return m.findByEmail(e)
}
func (m mockProfileRepo) FindByID(_ context.Context, id int64) (*userModels.GuardianProfile, error) {
	return m.findByID(id)
}
func (m mockProfileRepo) Create(_ context.Context, p *userModels.GuardianProfile) error {
	return m.create(p)
}
func (m mockProfileRepo) Delete(_ context.Context, id int64) error { return m.del(id) }

type mockLinkRepo struct {
	userModels.StudentGuardianRepository
	linkIfNotExists func(*userModels.StudentGuardian) (bool, error)
	findByGuardian  func(int64) ([]*userModels.StudentGuardian, error)
	findByStudent   func(int64) ([]*userModels.StudentGuardian, error)
}

func (m mockLinkRepo) LinkIfNotExists(_ context.Context, rel *userModels.StudentGuardian) (bool, error) {
	return m.linkIfNotExists(rel)
}
func (m mockLinkRepo) FindByGuardianProfileID(_ context.Context, id int64) ([]*userModels.StudentGuardian, error) {
	return m.findByGuardian(id)
}
func (m mockLinkRepo) FindByStudentID(_ context.Context, id int64) ([]*userModels.StudentGuardian, error) {
	return m.findByStudent(id)
}

type mockStudentRepo struct {
	userModels.StudentRepository
	findByID func(any) (*userModels.Student, error)
}

func (m mockStudentRepo) FindByID(_ context.Context, id any) (*userModels.Student, error) {
	return m.findByID(id)
}

type mockAccountRepo struct {
	authModels.AccountRepository
	findByID func(any) (*authModels.Account, error)
}

func (m mockAccountRepo) FindByID(_ context.Context, id any) (*authModels.Account, error) {
	return m.findByID(id)
}

// buildMockService wires the service with the provided mocks; nil mocks are
// left as the zero interface (only set what a test needs).
func buildMockService(cfg authService.GuardianInvitationServiceConfig) authService.GuardianInvitationService {
	cfg.FallbackExpiry = time.Hour
	return authService.NewGuardianInvitationService(cfg)
}

func pendingInvitation(profileID int64) *authModels.GuardianInvitation {
	inv := &authModels.GuardianInvitation{
		GuardianProfileID:           profileID,
		ApprovalStatus:              authModels.GuardianInvitationApprovalPending,
		ProfileCreatedForInvitation: true,
		ExpiresAt:                   time.Now().Add(time.Hour),
	}
	inv.ID = 99
	return inv
}

func TestInviteToStudent_RepoErrorsSurface(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	req := authService.InviteToStudentRequest{StudentID: 5, Email: "a@b.de", CreatedBy: 7}

	t.Run("profile create fails", func(t *testing.T) {
		svc := buildMockService(authService.GuardianInvitationServiceConfig{
			GuardianProfileRepo: mockProfileRepo{
				findByEmail: func(string) (*userModels.GuardianProfile, error) { return nil, nil },
				create:      func(*userModels.GuardianProfile) error { return errBoom },
			},
		})
		_, err := svc.InviteToStudent(ctx, req)
		require.Error(t, err)
	})

	t.Run("link fails", func(t *testing.T) {
		svc := buildMockService(authService.GuardianInvitationServiceConfig{
			GuardianProfileRepo: mockProfileRepo{
				findByEmail: func(string) (*userModels.GuardianProfile, error) { return nil, nil },
				create:      func(p *userModels.GuardianProfile) error { p.ID = 10; return nil },
			},
			StudentGuardianRepo: mockLinkRepo{
				linkIfNotExists: func(*userModels.StudentGuardian) (bool, error) { return false, errBoom },
			},
		})
		_, err := svc.InviteToStudent(ctx, req)
		require.Error(t, err)
	})

	t.Run("invitation create fails", func(t *testing.T) {
		svc := buildMockService(authService.GuardianInvitationServiceConfig{
			GuardianProfileRepo: mockProfileRepo{
				findByEmail: func(string) (*userModels.GuardianProfile, error) { return nil, nil },
				create:      func(p *userModels.GuardianProfile) error { p.ID = 10; return nil },
			},
			StudentGuardianRepo: mockLinkRepo{
				linkIfNotExists: func(*userModels.StudentGuardian) (bool, error) { return true, nil },
			},
			InvitationRepo: mockInvitationRepo{
				create: func(*authModels.GuardianInvitation) error { return errBoom },
			},
		})
		_, err := svc.InviteToStudent(ctx, req)
		require.Error(t, err)
	})
}

func TestListPendingApprovals_RepoErrorSurfaces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := buildMockService(authService.GuardianInvitationServiceConfig{
		InvitationRepo: mockInvitationRepo{
			findPendingApprv: func() ([]*authModels.GuardianInvitation, error) { return nil, errBoom },
		},
	})
	_, err := svc.ListPendingApprovalsDetailed(ctx)
	require.Error(t, err)
}

func TestApproveInvitation_ProfileLookupErrorSurfaces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := buildMockService(authService.GuardianInvitationServiceConfig{
		InvitationRepo: mockInvitationRepo{
			findByID: func(int64) (*authModels.GuardianInvitation, error) { return pendingInvitation(10), nil },
		},
		GuardianProfileRepo: mockProfileRepo{
			findByID: func(int64) (*userModels.GuardianProfile, error) { return nil, errBoom },
		},
	})
	require.Error(t, svc.ApproveInvitation(ctx, 99, 7))
}

func TestRejectInvitation_UpdateErrorSurfaces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := buildMockService(authService.GuardianInvitationServiceConfig{
		InvitationRepo: mockInvitationRepo{
			findByID: func(int64) (*authModels.GuardianInvitation, error) { return pendingInvitation(10), nil },
			update:   func(*authModels.GuardianInvitation) error { return errBoom },
		},
	})
	require.Error(t, svc.RejectInvitation(ctx, 99, 7))
}

func TestRejectInvitation_CleanupBranchesAreBestEffort(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := func(profileRepo userModels.GuardianProfileRepository, linkRepo userModels.StudentGuardianRepository) authService.GuardianInvitationService {
		return buildMockService(authService.GuardianInvitationServiceConfig{
			InvitationRepo: mockInvitationRepo{
				findByID: func(int64) (*authModels.GuardianInvitation, error) { return pendingInvitation(10), nil },
				update:   func(*authModels.GuardianInvitation) error { return nil },
			},
			GuardianProfileRepo: profileRepo,
			StudentGuardianRepo: linkRepo,
		})
	}
	noAccount := &userModels.GuardianProfile{HasAccount: false}

	t.Run("link check error is swallowed", func(t *testing.T) {
		svc := base(
			mockProfileRepo{findByID: func(int64) (*userModels.GuardianProfile, error) { return noAccount, nil }},
			mockLinkRepo{findByGuardian: func(int64) ([]*userModels.StudentGuardian, error) { return nil, errBoom }},
		)
		require.NoError(t, svc.RejectInvitation(ctx, 99, 7))
	})

	t.Run("delete error is swallowed", func(t *testing.T) {
		svc := base(
			mockProfileRepo{
				findByID: func(int64) (*userModels.GuardianProfile, error) { return noAccount, nil },
				del:      func(int64) error { return errBoom },
			},
			mockLinkRepo{findByGuardian: func(int64) ([]*userModels.StudentGuardian, error) { return nil, nil }},
		)
		require.NoError(t, svc.RejectInvitation(ctx, 99, 7))
	})

	t.Run("profile with account is left untouched", func(t *testing.T) {
		svc := base(
			mockProfileRepo{findByID: func(int64) (*userModels.GuardianProfile, error) {
				return &userModels.GuardianProfile{HasAccount: true}, nil
			}},
			mockLinkRepo{},
		)
		require.NoError(t, svc.RejectInvitation(ctx, 99, 7))
	})
}

func TestListPendingApprovalsDetailed_NameResolutionIsBestEffort(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sid := int64(5)
	rid := int64(7)
	inv := &authModels.GuardianInvitation{GuardianProfileID: 10, StudentID: &sid, RequestedByAccountID: &rid, ApprovalStatus: authModels.GuardianInvitationApprovalPending}
	inv.ID = 99

	svc := buildMockService(authService.GuardianInvitationServiceConfig{
		InvitationRepo: mockInvitationRepo{
			findPendingApprv: func() ([]*authModels.GuardianInvitation, error) {
				return []*authModels.GuardianInvitation{inv}, nil
			},
		},
		GuardianProfileRepo: mockProfileRepo{
			findByID: func(int64) (*userModels.GuardianProfile, error) { return nil, errBoom },
		},
		StudentRepo: mockStudentRepo{findByID: func(any) (*userModels.Student, error) { return nil, errBoom }},
		AccountRepo: mockAccountRepo{findByID: func(any) (*authModels.Account, error) { return nil, errBoom }},
	})

	views, err := svc.ListPendingApprovalsDetailed(ctx)
	require.NoError(t, err)
	require.Len(t, views, 1)
	// All lookups failed → names blank, but the row is still returned.
	assert.Empty(t, views[0].GuardianName)
	assert.Empty(t, views[0].StudentName)
	assert.Empty(t, views[0].RequestedByEmail)
}

func TestRevokeAccess_LinkLookupErrorSurfaces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := buildMockService(authService.GuardianInvitationServiceConfig{
		StudentGuardianRepo: mockLinkRepo{
			findByStudent: func(int64) ([]*userModels.StudentGuardian, error) { return nil, errBoom },
		},
	})
	err := svc.RevokeAccess(ctx, authService.RevokeAccessRequest{StudentID: 5, GuardianProfileID: 10, ActorAccountID: 7})
	require.Error(t, err)
}
