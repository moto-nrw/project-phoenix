package messaging

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

var (
	// ErrGroupThreadNotFound means no message of the group thread reached
	// the guardian for a child they may still open.
	ErrGroupThreadNotFound = errors.New("parent: group thread not found")
	// ErrGroupThreadClosed means the OGS closed the group thread: no answers.
	ErrGroupThreadClosed = errors.New("parent: group thread closed")
)

// GroupThread is a group thread (#4784) as a guardian sees it: the OGS
// writes to a group, class or AG; the guardian answers only the OGS.
type GroupThread struct {
	ID            int64
	TenantID      int64
	Title         string
	TargetLabel   string
	ClosedAt      *time.Time
	LastMessageAt *time.Time
}

// GroupMessage is one OGS message of a group thread. The sender is always
// the OGS; the staff member stays anonymous.
type GroupMessage struct {
	ID        int64
	Body      string
	CreatedAt time.Time
}

// GroupThreadSummary is one row of the guardian's group thread list.
type GroupThreadSummary struct {
	Thread          GroupThread
	LastMessageBody string
	LastMessageAt   *time.Time
	UnreadCount     int
	StudentIDs      []int64
}

// GroupThreadView is one opened group thread with the messages that reached
// the guardian and the children they got them for.
type GroupThreadView struct {
	Thread     GroupThread
	Messages   []GroupMessage
	StudentIDs []int64
}

// GroupThreads is the consumer-owned port to Communication's group threads.
// Every read is limited to recipient rows of the guardian for the given
// children; nothing describes another family.
type GroupThreads interface {
	ListGroupThreads(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) ([]GroupThreadSummary, error)
	CountUnreadGroupThreads(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) (int, error)
	ReadGroupThread(ctx context.Context, threadID, accountID int64, studentIDs []int64) (*GroupThreadView, bool, error)
	GroupReplyTarget(ctx context.Context, threadID, accountID, studentID int64, sendIDs ...string) (*GroupThread, error)
	GroupThreadTitles(ctx context.Context, threadIDs []int64) (map[int64]string, error)
}

// GroupThreadListItem is a list row with the school and the guardian's
// children the thread reached.
type GroupThreadListItem struct {
	GroupThreadSummary
	SchoolName string
	Children   []*care.ChildSummary
}

// GroupThreadDetail is an opened group thread with its school and children.
type GroupThreadDetail struct {
	GroupThreadView
	SchoolName string
	Children   []*care.ChildSummary
}

// portalChildren keeps the children the guardian may open in the portal:
// the relationship grants parent_portal.access. Only for them may a group
// thread be read.
func portalChildren(children []*care.ChildSummary) []*care.ChildSummary {
	out := make([]*care.ChildSummary, 0, len(children))
	for _, child := range children {
		if childGrants(child, care.GuardianPermissionPortalAccess) {
			out = append(out, child)
		}
	}
	return out
}

// childrenBySchool groups the children's ids per school.
func childrenBySchool(children []*care.ChildSummary) map[int64][]int64 {
	out := make(map[int64][]int64)
	for _, child := range children {
		out[child.TenantID] = append(out[child.TenantID], child.StudentID)
	}
	return out
}

func childIndex(children []*care.ChildSummary) (map[int64]*care.ChildSummary, []int64, []int64) {
	byID := make(map[int64]*care.ChildSummary, len(children))
	studentIDs := make([]int64, 0, len(children))
	for _, child := range children {
		byID[child.StudentID] = child
		studentIDs = append(studentIDs, child.StudentID)
	}
	return byID, studentIDs, distinctTenantIDs(children)
}

func pickChildren(byID map[int64]*care.ChildSummary, ids []int64) ([]*care.ChildSummary, string) {
	out := make([]*care.ChildSummary, 0, len(ids))
	school := ""
	for _, id := range ids {
		if child, ok := byID[id]; ok {
			out = append(out, child)
			school = child.SchoolName
		}
	}
	return out, school
}

// ListGroupThreads returns the guardian's group threads across schools,
// newest first. children is the guardian's child list; each school is read
// in its own tenant transaction.
func (s *Service) ListGroupThreads(ctx context.Context, accountID int64, children []*care.ChildSummary) ([]*GroupThreadListItem, error) {
	children = portalChildren(children)
	byID, _, _ := childIndex(children)
	out := make([]*GroupThreadListItem, 0)
	if s.GroupThreads == nil {
		return out, nil
	}
	for tenantID, studentIDs := range childrenBySchool(children) {
		var rows []GroupThreadSummary
		if err := s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
			var listErr error
			rows, listErr = s.GroupThreads.ListGroupThreads(txCtx, accountID, studentIDs, []int64{tenantID})
			return listErr
		}); err != nil {
			return nil, fmt.Errorf("parent: list group threads: %w", err)
		}
		enabled := s.MessagingEnabled(ctx, tenantID)
		for _, row := range rows {
			item := &GroupThreadListItem{GroupThreadSummary: row}
			item.Children, item.SchoolName = pickChildren(byID, row.StudentIDs)
			if !enabled {
				item.UnreadCount = 0
			}
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return groupThreadNewer(out[i], out[j]) })
	return out, nil
}

func groupThreadNewer(a, b *GroupThreadListItem) bool {
	if a.LastMessageAt == nil || b.LastMessageAt == nil {
		return a.LastMessageAt != nil
	}
	if !a.LastMessageAt.Equal(*b.LastMessageAt) {
		return a.LastMessageAt.After(*b.LastMessageAt)
	}
	return a.Thread.ID > b.Thread.ID
}

// unreadGroupThreadCount counts the guardian's group threads with an
// unread message in the given (messaging-enabled) schools.
func (s *Service) unreadGroupThreadCount(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) (int, error) {
	if s.GroupThreads == nil || len(studentIDs) == 0 || len(tenantIDs) == 0 {
		return 0, nil
	}
	return s.GroupThreads.CountUnreadGroupThreads(ctx, accountID, studentIDs, tenantIDs)
}

// GetGroupThread opens one group thread for the guardian and marks what
// they see read. children is the guardian's child list.
func (s *Service) GetGroupThread(ctx context.Context, accountID, threadID int64, children []*care.ChildSummary) (*GroupThreadDetail, error) {
	if s.GroupThreads == nil {
		return nil, ErrGroupThreadNotFound
	}
	children = portalChildren(children)
	byID, _, _ := childIndex(children)
	// A thread belongs to one school; read it there under RLS, with the
	// guardian's children of that school only.
	for tenantID, studentIDs := range childrenBySchool(children) {
		var view *GroupThreadView
		var advanced bool
		err := s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
			var readErr error
			view, advanced, readErr = s.GroupThreads.ReadGroupThread(txCtx, threadID, accountID, studentIDs)
			return readErr
		})
		if errors.Is(err, ErrGroupThreadNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("parent: get group thread: %w", err)
		}
		if advanced {
			// WithinTenant committed the cursor before returning. The staff group-
			// thread detail subscribes to this existing read receipt trigger, which
			// deliberately carries no identifiers through the staff SSE fan-out.
			s.broadcastReadReceipt(tenantID, accountID, conversationRef{Audience: AudienceTeam}, 0)
		}
		detail := &GroupThreadDetail{GroupThreadView: *view}
		detail.Children, detail.SchoolName = pickChildren(byID, view.StudentIDs)
		return detail, nil
	}
	return nil, ErrGroupThreadNotFound
}

// groupReplyRef returns the group thread reference of an answer, nil for a
// plain message.
func (s *Service) groupReplyRef(ctx context.Context, child *Child, accountID, studentID, threadID int64) (*int64, error) {
	if threadID <= 0 {
		return nil, nil
	}
	if err := s.requireGroupReply(ctx, child, accountID, studentID, threadID); err != nil {
		return nil, err
	}
	return &threadID, nil
}

// requireGroupReply checks that the guardian may answer the group thread
// for the child: it reached them for this child and is still open. The
// caller has already resolved the child under parent_portal.notes.write.
func (s *Service) requireGroupReply(ctx context.Context, child *Child, accountID, studentID, threadID int64) error {
	if s.GroupThreads == nil {
		return ErrGroupThreadNotFound
	}
	return s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		_, err := s.GroupThreads.GroupReplyTarget(txCtx, threadID, accountID, studentID, messageSendID(ctx))
		return err
	})
}

// decorateGroupThreadTitles names the group thread each guardian answer
// refers to. It runs inside the child's tenant transaction.
func (s *Service) decorateGroupThreadTitles(ctx context.Context, messages []*ParentMessage) error {
	if s.GroupThreads == nil {
		return nil
	}
	ids := make([]int64, 0)
	for _, message := range messages {
		if message != nil && message.GroupThreadID != nil {
			ids = append(ids, *message.GroupThreadID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	titles, err := s.GroupThreads.GroupThreadTitles(ctx, ids)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if message != nil && message.GroupThreadID != nil {
			message.GroupThreadTitle = titles[*message.GroupThreadID]
		}
	}
	return nil
}

func childGrants(child *care.ChildSummary, permission string) bool {
	return child != nil && child.GuardianPermissions[permission]
}
