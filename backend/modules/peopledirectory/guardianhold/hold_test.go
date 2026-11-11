package guardianhold_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/guardianhold"
)

// fakeFamily records the locks in the order Hold takes them.
type fakeFamily struct {
	calls      []string
	member     bool
	linked     bool
	granted    map[string]bool
	account    *int64
	email      string
	missing    map[int64]bool
	childError error
}

func newFamily(account int64) *fakeFamily {
	return &fakeFamily{
		member: true, linked: true, missing: map[int64]bool{},
		granted: map[string]bool{"parent_portal.access": true, "parent_portal.enrollment.submit": true},
		account: &account, email: "Eva@Example.test",
	}
}

func (f *fakeFamily) locks() guardianhold.Locks {
	return guardianhold.Locks{
		Membership: func(_ context.Context, account int64) (bool, error) {
			f.calls = append(f.calls, fmt.Sprintf("membership %d", account))
			return f.member, nil
		},
		Child: func(_ context.Context, student int64) error {
			f.calls = append(f.calls, fmt.Sprintf("child %d", student))
			return f.childError
		},
		Profile: func(_ context.Context, profile int64) (bool, error) {
			f.calls = append(f.calls, fmt.Sprintf("profile %d", profile))
			return !f.missing[profile], nil
		},
		Relationship: func(_ context.Context, student, profile int64) (func(string) bool, error) {
			f.calls = append(f.calls, fmt.Sprintf("relationship %d/%d", student, profile))
			if !f.linked {
				return nil, nil
			}
			return func(permission string) bool { return f.granted[permission] }, nil
		},
		Owner: func(_ context.Context, profile int64) (*int64, string, bool, error) {
			f.calls = append(f.calls, fmt.Sprintf("owner %d", profile))
			if f.missing[profile] {
				return nil, "", false, nil
			}
			return f.account, f.email, true, nil
		},
	}
}

func TestHoldLocksAccountSchoolChildProfilesRelationshipInOrder(t *testing.T) {
	t.Parallel()
	f := newFamily(7)
	err := guardianhold.Hold(context.Background(), f.locks(), guardianhold.Request{
		AccountID: 7, StudentID: 3, ProfileID: 20, OwnProfile: true, Profiles: []int64{30, 10, 30},
		Access: "parent_portal.access", Permissions: []string{"parent_portal.enrollment.submit"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"membership 7", "child 3", "profile 10", "profile 20", "profile 30", "relationship 3/20", "owner 20"}, f.calls)
}

// A sibling renewal must hold all child rows, even when its email profile
// carries an account, before it can block a contact edit on that profile.
func TestHoldLocksAllCandidateChildrenBeforeProfiles(t *testing.T) {
	t.Parallel()
	f := newFamily(7)
	others := []int64{9, 3, 5, 3}
	err := guardianhold.Hold(context.Background(), f.locks(), guardianhold.Request{
		Email: f.email, StudentID: 5, ProfileID: 20,
		Permissions: []string{"parent_portal.enrollment.submit"},
	}, others...)
	require.NoError(t, err)
	assert.Equal(t, []string{"owner 20", "membership 7", "child 3", "child 5", "child 9", "profile 20", "relationship 5/20", "owner 20"}, f.calls)
	assert.Equal(t, []int64{9, 3, 5, 3}, others, "the caller's candidate order is unchanged")
}

func TestHoldRefusesWhatTheIdentityNoLongerHolds(t *testing.T) {
	t.Parallel()
	base := guardianhold.Request{AccountID: 7, StudentID: 3, ProfileID: 20, Access: "parent_portal.access", Permissions: []string{"parent_portal.enrollment.submit"}}
	for _, tc := range []struct {
		name   string
		change func(*fakeFamily)
		want   error
	}{
		{"Schulmitgliedschaft", func(f *fakeFamily) { f.member = false }, guardianhold.ErrNotLinked},
		{"Beziehung", func(f *fakeFamily) { f.linked = false }, guardianhold.ErrNotLinked},
		{"Kontobindung", func(f *fakeFamily) { moved := *f.account + 1; f.account = &moved }, guardianhold.ErrNotLinked},
		{"Portalzugang", func(f *fakeFamily) { f.granted["parent_portal.access"] = false }, guardianhold.ErrNotLinked},
		{"Aktionsrecht", func(f *fakeFamily) { f.granted["parent_portal.enrollment.submit"] = false }, guardianhold.ErrPermissionDenied},
		{"eigenes Profil gelöscht", func(f *fakeFamily) { f.missing[20] = true }, guardianhold.ErrNotLinked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFamily(7)
			tc.change(f)
			require.ErrorIs(t, guardianhold.Hold(context.Background(), f.locks(), base), tc.want)
		})
	}
}

func TestHoldPassesTheChildLockErrorThrough(t *testing.T) {
	t.Parallel()
	f := newFamily(7)
	gone := errors.New("child deleted")
	f.childError = gone
	err := guardianhold.Hold(context.Background(), f.locks(), guardianhold.Request{AccountID: 7, StudentID: 3, ProfileID: 20})
	require.ErrorIs(t, err, gone)
}

func TestHoldLeavesAMissingFurtherProfileToTheWrite(t *testing.T) {
	t.Parallel()
	f := newFamily(7)
	f.missing[30] = true
	err := guardianhold.Hold(context.Background(), f.locks(), guardianhold.Request{AccountID: 7, StudentID: 3, ProfileID: 20, Profiles: []int64{30}})
	require.NoError(t, err)
}

// An accountless guardian is named by the address on its profile: Hold
// locks that profile, asks no membership while it has no account and
// refuses once the address changed.
func TestHoldAccountlessGuardianByAddress(t *testing.T) {
	t.Parallel()
	req := guardianhold.Request{Email: " eva@example.test", StudentID: 3, ProfileID: 20, Permissions: []string{"parent_portal.enrollment.submit"}}

	f := newFamily(0)
	f.account = nil
	require.NoError(t, guardianhold.Hold(context.Background(), f.locks(), req))
	assert.Equal(t, []string{"owner 20", "child 3", "profile 20", "relationship 3/20", "owner 20"}, f.calls)

	f = newFamily(0)
	f.account = nil
	f.email = "neu@example.test"
	require.ErrorIs(t, guardianhold.Hold(context.Background(), f.locks(), req), guardianhold.ErrNotLinked)

	// A profile that carries an account needs that account's membership.
	f = newFamily(9)
	require.NoError(t, guardianhold.Hold(context.Background(), f.locks(), req))
	assert.Equal(t, "membership 9", f.calls[1])
	f = newFamily(9)
	f.member = false
	require.ErrorIs(t, guardianhold.Hold(context.Background(), f.locks(), req), guardianhold.ErrNotLinked)
}

func TestHoldFailsClosed(t *testing.T) {
	t.Parallel()
	f := newFamily(7)
	locks := f.locks()
	locks.Membership = nil
	require.ErrorIs(t, guardianhold.Hold(context.Background(), locks, guardianhold.Request{AccountID: 7, StudentID: 3, ProfileID: 20}), guardianhold.ErrUnavailable)
	require.ErrorIs(t, guardianhold.Hold(context.Background(), f.locks(), guardianhold.Request{StudentID: 3, ProfileID: 20}), guardianhold.ErrNotLinked)
	assert.Empty(t, f.calls, "an unidentified request locks nothing")
}
