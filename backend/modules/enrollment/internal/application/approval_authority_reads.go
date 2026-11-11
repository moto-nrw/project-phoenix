package application

import (
	"context"
	"fmt"
	"maps"
	"slices"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type approvalAuthorityReadsKey struct{}

// approvalAuthorityReads belongs to one approval transaction. Identity
// reads and grants acquired by the parent guard are reused until commit.
type approvalAuthorityReads struct {
	origins map[int64]*enrollmentNative.Request
	invites map[int64]*enrollmentNative.LateInvite
	holds   map[approvalAuthorityKey]renewalHolds
}

// approvalAuthorityKey keeps held grants scoped to the school and identity.
type approvalAuthorityKey struct {
	tenantID  int64
	submitter string
}

func (s *ChangeRequests) loadApprovalAuthorityReads(ctx context.Context, run *approval) (*approvalAuthorityReads, error) {
	var children []*enrollmentNative.RequestChild
	needsIdentity := false
	for _, child := range run.children {
		syncs := s.approvedChildUsesDecisionSync(child) && !child.MatchedByOffice
		needsIdentity = needsIdentity || syncs || reconcilesPin(run.prepared.phase, child)
		if syncs && run.req.GuardianAccountID == nil && child.RolloverSourceChildID != nil {
			children = append(children, child)
		}
	}
	origins, err := loadRolloverOrigins(ctx, s.deps.Children, s.deps.Requests, children)
	if err != nil {
		return nil, err
	}
	identities := slices.Collect(maps.Values(origins))
	if needsIdentity {
		identities = append(identities, run.req)
	}
	invites, err := s.loadApprovalInvites(ctx, identities)
	if err != nil {
		return nil, err
	}
	return &approvalAuthorityReads{origins: origins, invites: invites, holds: map[approvalAuthorityKey]renewalHolds{}}, nil
}

func (s *ChangeRequests) loadApprovalInvites(ctx context.Context, identities []*enrollmentNative.Request) (map[int64]*enrollmentNative.LateInvite, error) {
	byRequest := map[int64]*enrollmentNative.LateInvite{}
	for _, req := range identities {
		if req != nil && req.GuardianAccountID == nil && enrollmentNative.NormalizedSubmissionSource(req.SubmissionSource) == enrollmentNative.RequestSourceLateInvite {
			byRequest[req.ID] = nil
		}
	}
	if len(byRequest) == 0 {
		return byRequest, nil
	}
	if s.deps.LateInviteIdentities == nil {
		return nil, fmt.Errorf("late invite repository is not configured")
	}
	invites, err := s.deps.LateInviteIdentities.LateInvitesByUsedRequestIDs(ctx, slices.Sorted(maps.Keys(byRequest)))
	if err != nil {
		return nil, fmt.Errorf("change request approve: load late invite identities: %w", err)
	}
	for _, invite := range invites {
		if invite.UsedRequestID != nil {
			byRequest[*invite.UsedRequestID] = invite
		}
	}
	return byRequest, nil
}

// loadRolloverOrigins reads all ancestry one generation at a time, then
// loads the distinct origin requests once. Missing sources cut a chain;
// cycles and overlong chains keep the single-child walk's depth limit.
func loadRolloverOrigins(ctx context.Context, children interface {
	ChildrenByID(context.Context, []int64) ([]*enrollmentNative.RequestChild, error)
}, requests interface {
	RequestsByID(context.Context, []int64) ([]*enrollmentNative.Request, error)
}, starts []*enrollmentNative.RequestChild) (map[int64]*enrollmentNative.Request, error) {
	current := map[int64]*enrollmentNative.RequestChild{}
	loaded := map[int64]*enrollmentNative.RequestChild{}
	origins := map[int64]*enrollmentNative.Request{}
	requestIDs := map[int64]int64{}
	for _, child := range starts {
		current[child.ID], loaded[child.ID] = child, child
		origins[child.ID] = nil
	}
	for depth := 0; len(current) > 0; depth++ {
		ids, err := rolloverGeneration(current, loaded, requestIDs, depth)
		if err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			rows, err := children.ChildrenByID(ctx, ids)
			if err != nil {
				return nil, fmt.Errorf("decision: load rollover sources: %w", err)
			}
			for _, row := range rows {
				loaded[row.ID] = row
			}
		}
		advanceRolloverGeneration(current, loaded)
	}
	return loadRolloverRequests(ctx, requests, origins, requestIDs)
}

func advanceRolloverGeneration(current, loaded map[int64]*enrollmentNative.RequestChild) {
	for id, source := range current {
		next := loaded[*source.RolloverSourceChildID]
		if next == nil {
			delete(current, id)
		} else {
			current[id] = next
		}
	}
}

func loadRolloverRequests(ctx context.Context, requests interface {
	RequestsByID(context.Context, []int64) ([]*enrollmentNative.Request, error)
}, origins map[int64]*enrollmentNative.Request, requestIDs map[int64]int64) (map[int64]*enrollmentNative.Request, error) {
	if len(requestIDs) == 0 {
		return origins, nil
	}
	ids := slices.Sorted(maps.Values(requestIDs))
	rows, err := requests.RequestsByID(ctx, slices.Compact(ids))
	if err != nil {
		return nil, fmt.Errorf("decision: load rollover origin requests: %w", err)
	}
	byID := map[int64]*enrollmentNative.Request{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	for childID, requestID := range requestIDs {
		origins[childID] = byID[requestID]
	}
	return origins, nil
}

func rolloverGeneration(current, loaded map[int64]*enrollmentNative.RequestChild, requestIDs map[int64]int64, depth int) ([]int64, error) {
	var ids []int64
	for id, source := range current {
		if source.RolloverSourceChildID == nil {
			requestIDs[id] = source.RequestID
			delete(current, id)
			continue
		}
		if depth >= maxRolloverDepth {
			return nil, fmt.Errorf("decision: rollover chain of child %d exceeds %d phases", id, maxRolloverDepth)
		}
		if _, ok := loaded[*source.RolloverSourceChildID]; !ok {
			ids = append(ids, *source.RolloverSourceChildID)
			loaded[*source.RolloverSourceChildID] = nil
		}
	}
	slices.Sort(ids)
	return ids, nil
}

func approvalRolloverOrigin(ctx context.Context, childID int64) (*enrollmentNative.Request, bool) {
	reads, _ := ctx.Value(approvalAuthorityReadsKey{}).(*approvalAuthorityReads)
	if reads == nil {
		return nil, false
	}
	origin, ok := reads.origins[childID]
	return origin, ok
}

func approvalLateInvite(ctx context.Context, lateInvites DecisionLateInvites, requestID int64) (*enrollmentNative.LateInvite, error) {
	if reads, _ := ctx.Value(approvalAuthorityReadsKey{}).(*approvalAuthorityReads); reads != nil {
		if invite, ok := reads.invites[requestID]; ok {
			if invite == nil {
				return nil, enrollmentNative.ErrLateInviteNotFound
			}
			return invite, nil
		}
	}
	return lateInvites.LateInviteByUsedRequestID(ctx, requestID)
}

// holdApprovalAuthority resolves the pins the approval leaves and holds the
// parent authority every later check of the approval asks, behind the parent
// guard (#6638): the pins against the request's identity after the edit, as
// the pin check always asked, and every approved student the decision flow
// syncs against the identity that sync asks (syncAuthorityRequest). It runs
// before the dedup and match locks, in the guard's order: account, school,
// the approved graph with every held student, profile, relationships. The
// graph is locked again afterwards, which costs no wait once the hold took
// it and locks it when no identity is held.
func (s *ChangeRequests) holdApprovalAuthority(ctx context.Context, run *approval) error {
	reads, err := s.loadApprovalAuthorityReads(ctx, run)
	if err != nil {
		return err
	}
	run.reads = reads
	ctx = context.WithValue(ctx, approvalAuthorityReadsKey{}, reads)
	authority := newApprovalAuthority(s, run)
	for i, child := range run.children {
		if err := authority.collect(ctx, i, child); err != nil {
			return err
		}
	}
	if err := authority.hold(ctx); err != nil {
		return err
	}
	return s.lockApprovedStudents(ctx, run.children, authority.students())
}
