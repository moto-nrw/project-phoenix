package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

// Group thread values and port (#4784). The root binds the port to
// Communication's guardian side of the group threads.
type (
	GroupThreads        = messaging.GroupThreads
	GroupThread         = messaging.GroupThread
	GroupMessage        = messaging.GroupMessage
	GroupThreadSummary  = messaging.GroupThreadSummary
	GroupThreadView     = messaging.GroupThreadView
	GroupThreadListItem = messaging.GroupThreadListItem
	GroupThreadDetail   = messaging.GroupThreadDetail
)

var (
	ErrGroupThreadNotFound = messaging.ErrGroupThreadNotFound
	ErrGroupThreadClosed   = messaging.ErrGroupThreadClosed
)

// ListGroupThreads resolves the guardian's children through the child list
// and reads the group threads that reached them.
func (p *Portal) ListGroupThreads(ctx context.Context, accountID int64) ([]*GroupThreadListItem, error) {
	children, err := p.ListChildrenForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return p.MessagingFlows.ListGroupThreads(ctx, accountID, children)
}

// GetGroupThread opens one group thread for the guardian's children.
func (p *Portal) GetGroupThread(ctx context.Context, accountID, threadID int64) (*GroupThreadDetail, error) {
	children, err := p.ListChildrenForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return p.MessagingFlows.GetGroupThread(ctx, accountID, threadID, children)
}

// PostChildReply writes the guardian's message to the child's conversation
// of the optional audience (#5200), answering the group thread when positive.
func (p *Portal) PostChildReply(ctx context.Context, accountID, studentID int64, body string, groupThreadID int64, audience ...string) (*MessageThreadView, error) {
	target := MessageTarget{GroupThreadID: groupThreadID}
	if len(audience) > 0 {
		target.Audience = audience[0]
	}
	return p.MessagingFlows.PostChildMessage(ctx, accountID, studentID, body, target)
}
