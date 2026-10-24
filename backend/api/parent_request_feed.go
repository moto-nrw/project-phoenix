package api

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed"
	requestFeedCompose "github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/compose"
	parentRequestFeedView "github.com/moto-nrw/project-phoenix/modules/parentrequestfeedprojection/compose"
)

// parentRequestFeed binds Care Plan's request feed to the parent request feed
// projection (ADR 0058): a domain that needs a projection's result declares
// its reader port, and the root binds it. The projection reads in the feed's
// own transaction.
type parentRequestFeed struct{ view *parentRequestFeedView.Reader }

func newParentRequestFeed(database requestFeedCompose.Database) requestfeed.FeedSource {
	return parentRequestFeed{view: parentRequestFeedView.NewReader(parentRequestFeedView.Database(database))}
}

func (f parentRequestFeed) ResolveAccess(ctx context.Context, tenantID, accountID int64) (requestfeed.FeedAccess, bool, error) {
	access, found, err := f.view.ResolveAccess(ctx, tenantID, accountID)
	return requestfeed.FeedAccess{
		GeneralRequests: access.GeneralRequests, EnrollmentRequests: access.EnrollmentRequests,
		SchoolName: access.SchoolName, Subdomain: access.Subdomain,
	}, found, err
}

func (f parentRequestFeed) ListNewParentRequests(ctx context.Context, tenantID int64, since time.Time, generalRequests, enrollmentRequests bool) ([]requestfeed.FeedEntry, error) {
	entries, err := f.view.ListNewParentRequests(ctx, tenantID, since, generalRequests, enrollmentRequests)
	if err != nil {
		return nil, err
	}
	result := make([]requestfeed.FeedEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, requestfeed.FeedEntry{Kind: entry.Kind, ID: entry.ID, CreatedAt: entry.CreatedAt})
	}
	return result, nil
}
