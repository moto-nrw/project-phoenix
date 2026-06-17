package parent

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/uptrace/bun"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const maxMessageSubjectLen = 150

// MessageThreadView is the parent-facing chat payload: the thread header
// (subject, child) plus the messages, oldest-first.
type MessageThreadView struct {
	ThreadID    int64
	Subject     string
	StudentID   int64
	StudentName string
	Messages    []*usersModels.ParentMessage
}

// resolveOwnedThread loads a thread across tenants (admin tx) and verifies the
// calling account is its guardian. Returns ErrChildNotLinked when the thread
// does not exist or is not owned, so a guardian can never probe other threads.
func (s *service) resolveOwnedThread(ctx context.Context, accountID, threadID int64) (*usersModels.ParentMessageThread, error) {
	if accountID <= 0 || threadID <= 0 {
		return nil, ErrChildNotLinked
	}
	var resolved *usersModels.ParentMessageThread
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		thread, findErr := s.messageThreadRepo.FindByID(adminCtx, threadID)
		if findErr != nil {
			return findErr
		}
		if thread == nil || thread.GuardianAccountID != accountID {
			return ErrChildNotLinked
		}
		resolved = thread
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

// ListMessageThreads returns every thread the guardian owns across all their
// children's schools, newest activity first.
func (s *service) ListMessageThreads(ctx context.Context, accountID int64) ([]*usersModels.InboxThread, error) {
	children, err := s.ListChildrenForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	// Distinct tenants the guardian has children at.
	seen := make(map[int64]bool, len(children))
	tenantIDs := make([]int64, 0, len(children))
	for _, c := range children {
		if !seen[c.TenantID] {
			seen[c.TenantID] = true
			tenantIDs = append(tenantIDs, c.TenantID)
		}
	}

	out := make([]*usersModels.InboxThread, 0)
	for _, tid := range tenantIDs {
		txErr := tenant.WithTenantTx(ctx, s.db, tid, func(txCtx context.Context, _ bun.Tx) error {
			rows, err := s.messageReadRepo.ListThreadsForGuardian(txCtx, accountID)
			if err != nil {
				return err
			}
			out = append(out, rows...)
			return nil
		})
		if txErr != nil {
			return nil, fmt.Errorf("parent: list message threads: %w", txErr)
		}
	}
	return out, nil
}

// GetMessageThread returns the conversation and marks it read for the guardian.
func (s *service) GetMessageThread(ctx context.Context, accountID, threadID int64) (*MessageThreadView, error) {
	thread, err := s.resolveOwnedThread(ctx, accountID, threadID)
	if err != nil {
		return nil, err
	}

	view := &MessageThreadView{ThreadID: thread.ID, Subject: thread.Subject, StudentID: thread.StudentID}
	txErr := tenant.WithTenantTx(ctx, s.db, thread.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		messages, err := s.messageRepo.ListByThread(txCtx, thread.ID, 0)
		if err != nil {
			return err
		}
		view.Messages = messages
		if proj, err := s.messageReadRepo.FindThreadProjection(txCtx, thread.ID, accountID); err == nil && proj != nil {
			view.StudentName = proj.StudentName
		}
		return s.messageReadRepo.MarkRead(txCtx, thread.TenantID, thread.ID, accountID)
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: get message thread: %w", txErr)
	}
	return view, nil
}

// PostThreadMessage appends a guardian message to an existing owned thread.
func (s *service) PostThreadMessage(ctx context.Context, accountID, threadID int64, body string) (*MessageThreadView, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrEmptyNote
	}
	if utf8.RuneCountInString(body) > maxParentNoteLen {
		return nil, ErrNoteTooLong
	}
	thread, err := s.resolveOwnedThread(ctx, accountID, threadID)
	if err != nil {
		return nil, err
	}

	enabled, err := s.settings.ResolveBoolForTenant(ctx, thread.TenantID, configModels.KeyParentNotesEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve messaging setting: %w", err)
	}
	if !enabled {
		return nil, ErrNotesDisabled
	}

	senderName := s.resolveGuardianName(ctx, accountID)
	view := &MessageThreadView{ThreadID: thread.ID, Subject: thread.Subject, StudentID: thread.StudentID}
	txErr := tenant.WithTenantTx(ctx, s.db, thread.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.appendGuardianMessage(txCtx, thread, accountID, senderName, body); err != nil {
			return err
		}
		messages, err := s.messageRepo.ListByThread(txCtx, thread.ID, 0)
		if err != nil {
			return err
		}
		view.Messages = messages
		captured := thread.TenantID
		tenant.RegisterAfterCommit(txCtx, func() {
			s.broadcastParentMessage(captured, accountID)
		})
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: post thread message: %w", txErr)
	}
	return view, nil
}

// StartThread opens a new conversation about an owned child.
func (s *service) StartThread(ctx context.Context, accountID, studentID int64, subject, body string) (*MessageThreadView, error) {
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if subject == "" {
		return nil, ErrEmptyNote
	}
	if utf8.RuneCountInString(subject) > maxMessageSubjectLen {
		subject = string([]rune(subject)[:maxMessageSubjectLen])
	}
	if body == "" {
		return nil, ErrEmptyNote
	}
	if utf8.RuneCountInString(body) > maxParentNoteLen {
		return nil, ErrNoteTooLong
	}

	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	enabled, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentNotesEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve messaging setting: %w", err)
	}
	if !enabled {
		return nil, ErrNotesDisabled
	}

	senderName := s.resolveGuardianName(ctx, accountID)
	view := &MessageThreadView{Subject: subject, StudentID: studentID}
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		thread := &usersModels.ParentMessageThread{
			StudentID:         studentID,
			GuardianAccountID: accountID,
			Subject:           subject,
		}
		thread.SetTenantID(child.tenantID)
		if err := s.messageThreadRepo.Create(txCtx, thread); err != nil {
			return err
		}
		if err := s.appendGuardianMessage(txCtx, thread, accountID, senderName, body); err != nil {
			return err
		}
		view.ThreadID = thread.ID
		messages, err := s.messageRepo.ListByThread(txCtx, thread.ID, 0)
		if err != nil {
			return err
		}
		view.Messages = messages
		captured := child.tenantID
		tenant.RegisterAfterCommit(txCtx, func() {
			s.broadcastParentMessage(captured, accountID)
		})
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: start thread: %w", txErr)
	}
	s.logger.Info("parent started message thread",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
	)
	return view, nil
}

// MarkMessageThreadRead advances the guardian's read cursor for a thread.
func (s *service) MarkMessageThreadRead(ctx context.Context, accountID, threadID int64) error {
	thread, err := s.resolveOwnedThread(ctx, accountID, threadID)
	if err != nil {
		return err
	}
	txErr := tenant.WithTenantTx(ctx, s.db, thread.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.messageReadRepo.MarkRead(txCtx, thread.TenantID, thread.ID, accountID)
	})
	if txErr != nil {
		return fmt.Errorf("parent: mark thread read: %w", txErr)
	}
	return nil
}

// appendGuardianMessage writes a guardian message into the thread, updates the
// thread's last-activity fields, and marks it read for the sender.
func (s *service) appendGuardianMessage(ctx context.Context, thread *usersModels.ParentMessageThread, accountID int64, senderName, body string) error {
	msg := &usersModels.ParentMessage{
		ThreadID:        thread.ID,
		StudentID:       thread.StudentID,
		SenderAccountID: accountID,
		SenderKind:      usersModels.ParentMessageSenderGuardian,
		SenderName:      senderName,
		Body:            body,
	}
	msg.SetTenantID(thread.TenantID)
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return err
	}
	now := time.Now()
	kind := usersModels.ParentMessageSenderGuardian
	thread.LastMessageAt = &now
	thread.LastSenderKind = &kind
	if err := s.messageThreadRepo.Update(ctx, thread); err != nil {
		return err
	}
	return s.messageReadRepo.MarkRead(ctx, thread.TenantID, thread.ID, accountID)
}

// resolveGuardianName returns the guardian's display name, "Elternteil" if none.
func (s *service) resolveGuardianName(ctx context.Context, accountID int64) string {
	name := "Elternteil"
	if s.guardianProfileRepo == nil {
		return name
	}
	_ = tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		profile, err := s.guardianProfileRepo.FindByAccountID(adminCtx, accountID)
		if err != nil || profile == nil {
			return nil
		}
		if full := strings.TrimSpace(profile.FirstName + " " + profile.LastName); full != "" {
			name = full
		}
		return nil
	})
	return name
}

// broadcastParentMessage wakes the guardian's own tabs and the tenant's staff.
func (s *service) broadcastParentMessage(tenantID, guardianAccountID int64) {
	if s.broadcaster == nil || tenantID <= 0 {
		return
	}
	gid := strconv.FormatInt(guardianAccountID, 10)
	event := realtime.NewEvent(realtime.EventParentMessage, "", realtime.EventData{Source: &gid})
	if err := s.broadcaster.BroadcastParentMessage(tenantID, guardianAccountID, event); err != nil {
		s.logger.Warn("parent: failed to broadcast parent message",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("guardian_account_id", guardianAccountID),
			slog.String("error", err.Error()),
		)
	}
}
