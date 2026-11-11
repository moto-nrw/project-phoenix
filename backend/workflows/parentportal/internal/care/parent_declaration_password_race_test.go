package care_test

// #6494, problem 1: the password a declaration needs is the one the school
// asks for when the declaration is written, not the one the check before the
// write found. These tests park a guardian's declaration around its
// (announcement, child) lock and let the school retract the declaration,
// switch the password confirmation on and publish it again with the same
// wording, through the same store writes the staff edit takes. Sleeps are
// not used; the tests wait on channels and on pg_blocking_pids.

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// declarationPark parks the first declaration while armed and reports the
// database backend of its transaction.
type declarationPark struct {
	armed   atomic.Bool
	once    sync.Once
	opened  sync.Once
	reached chan parkedBackend
	release chan struct{}
}

type parkedBackend struct {
	pid int
	err error
}

func newDeclarationPark() *declarationPark {
	return &declarationPark{reached: make(chan parkedBackend, 1), release: make(chan struct{})}
}

func (p *declarationPark) open() { p.opened.Do(func() { close(p.release) }) }

func (p *declarationPark) park(ctx context.Context) {
	if !p.armed.Load() {
		return
	}
	p.once.Do(func() {
		var held parkedBackend
		if tx, ok := tenant.TransactionFromContext(ctx); !ok {
			held.err = errors.New("the declaration lock runs outside a transaction")
		} else {
			held.err = tx.(bun.IDB).NewRaw("SELECT pg_backend_pid()").Scan(ctx, &held.pid)
		}
		p.reached <- held
		<-p.release
	})
}

// parkedDeclarationLock parks a declaration right before and right after it
// takes the (announcement, child) lock.
type parkedDeclarationLock struct {
	parentportal.AnnouncementAccess
	before, after *declarationPark
}

func (l parkedDeclarationLock) LockDeclarationChild(ctx context.Context, announcementID, studentID int64) error {
	l.before.park(ctx)
	if err := l.AnnouncementAccess.LockDeclarationChild(ctx, announcementID, studentID); err != nil {
		return err
	}
	l.after.park(ctx)
	return nil
}

func newParkedDeclarationSetup(t *testing.T, settings communication.ParentDeclarationSettings) (*declarationSetup, *declarationPark, *declarationPark) {
	t.Helper()
	before, after := newDeclarationPark(), newDeclarationPark()
	t.Cleanup(before.open)
	t.Cleanup(after.open)
	s := newDeclarationSetupWith(t, settings, nil, func(access parentportal.AnnouncementAccess) parentportal.AnnouncementAccess {
		return parkedDeclarationLock{AnnouncementAccess: access, before: before, after: after}
	})
	return s, before, after
}

type declarationOutcome struct {
	submission *parentportal.DeclarationSubmission
	created    bool
	err        error
}

func (s *declarationSetup) submitAsync(in parentportal.DeclarationInput, confirm parentportal.PasswordConfirmer) <-chan declarationOutcome {
	done := make(chan declarationOutcome, 1)
	go func() {
		submission, created, err := s.svc.SubmitDeclaration(s.ctx, s.chain.AccountID, s.announcement.ID, in, confirm)
		done <- declarationOutcome{submission: submission, created: created, err: err}
	}()
	return done
}

func awaitPark(t *testing.T, p *declarationPark) int {
	t.Helper()
	select {
	case held := <-p.reached:
		require.NoError(t, held.err)
		return held.pid
	case <-time.After(20 * time.Second):
		t.Fatal("the declaration did not reach its lock")
		return 0
	}
}

func awaitDeclaration(t *testing.T, done <-chan declarationOutcome) declarationOutcome {
	t.Helper()
	select {
	case outcome := <-done:
		return outcome
	case <-time.After(20 * time.Second):
		t.Fatal("the declaration did not finish")
		return declarationOutcome{}
	}
}

// requirePassword is the staff correction: retract, switch the password
// confirmation on, publish again. Title, body and kind stay, so the frozen
// version stays the one the guardian loaded.
func (s *declarationSetup) requirePassword(t *testing.T) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	require.NoError(t, testpkg.TenantCommand2(t, s.seedCtx, db, s.repo.SetPublished, s.announcement.ID, nil))
	draft, err := testpkg.TenantQuery1(t, s.seedCtx, db, s.repo.FindByID, s.announcement.ID)
	require.NoError(t, err)
	draft.Declaration.RequiresPassword = true
	require.NoError(t, testpkg.TenantCommand1(t, s.seedCtx, db, s.repo.Update, draft))
	published := time.Now()
	require.NoError(t, testpkg.TenantCommand2(t, s.seedCtx, db, s.repo.SetPublished, s.announcement.ID, &published))
}

func TestDeclaration_PasswordSwitchedOnAfterTheCheckRefusesTheConsent(t *testing.T) {
	t.Parallel()
	s, before, _ := newParkedDeclarationSetup(t, consent(communication.DeclarationSignersAny, false))
	in := parentportal.DeclarationInput{StudentID: s.chain.StudentID, VersionID: s.version.ID, Action: parentportal.DeclarationActionAgreed}

	before.armed.Store(true)
	done := s.submitAsync(in, nil)
	awaitPark(t, before)
	s.requirePassword(t)
	before.open()

	outcome := awaitDeclaration(t, done)
	require.ErrorIs(t, outcome.err, parentportal.ErrDeclarationPasswordRequired,
		"the password the school asks for now must be confirmed")
	assert.Empty(t, s.history(t), "no consent and no proof without the password")

	// The guardian confirms the password and the consent is recorded.
	const secret = "richtig"
	in.Password = secret
	submission, created, err := s.svc.SubmitDeclaration(s.ctx, s.chain.AccountID, s.announcement.ID, in, func(_ context.Context, given string) error {
		if given != secret {
			return parentportal.ErrDeclarationPasswordIncorrect
		}
		return nil
	})
	require.NoError(t, err)
	assert.True(t, created)
	assert.True(t, submission.PasswordConfirmed)
	assert.Equal(t, s.version.ID, submission.VersionID)
}

// The declaration holds the announcement's settings until it commits: a
// retraction the school starts meanwhile waits, and afterwards finds the
// answer stored, so the settings guard of the staff edit sees it.
func TestDeclaration_RetractionWaitsForTheDeclarationThatHoldsIt(t *testing.T) {
	t.Parallel()
	s, _, after := newParkedDeclarationSetup(t, consent(communication.DeclarationSignersAny, false))
	in := parentportal.DeclarationInput{StudentID: s.chain.StudentID, VersionID: s.version.ID, Action: parentportal.DeclarationActionAgreed}
	db := testpkg.SetupTestDB(t)

	after.armed.Store(true)
	done := s.submitAsync(in, nil)
	pid := awaitPark(t, after)

	retracted := make(chan error, 1)
	go func() {
		retracted <- testpkg.TenantCommand2(t, s.seedCtx, db, s.repo.SetPublished, s.announcement.ID, nil)
	}()
	require.Eventually(t, func() bool {
		select {
		case err := <-retracted:
			retracted <- err
			return true
		default:
		}
		var waiting bool
		err := db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).
			Scan(context.Background(), &waiting)
		return err == nil && waiting
	}, 20*time.Second, 10*time.Millisecond)
	select {
	case err := <-retracted:
		t.Fatalf("the retraction committed while the declaration held the announcement: %v", err)
	default:
	}
	after.open()

	outcome := awaitDeclaration(t, done)
	require.NoError(t, outcome.err)
	assert.True(t, outcome.created)
	select {
	case err := <-retracted:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("the retraction did not finish")
	}
	assert.Len(t, s.history(t), 1)
}
