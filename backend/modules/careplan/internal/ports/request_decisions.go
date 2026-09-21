package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

type RequestDecisionRecords interface {
	Find(context.Context, int64, bool) (carerequests.Request, error)
	Decide(context.Context, int64, string, *string, int64, bool) error
	StoreSnapshot(context.Context, int64, *carerequests.DecisionSnapshot) error
}

type RequestDecisionPeople interface {
	LockStudent(context.Context, int64) (ReviewStudent, error)
	CanReview(context.Context, ReviewStudent) (bool, error)
	GuardianHasAccess(context.Context, int64, int64) (bool, error)
}

type RequestDecisionPlans interface {
	Snapshot(context.Context, *carerequests.Request) *carerequests.DecisionSnapshot
	ApplyWeekly(context.Context, *carerequests.Request, int64) (bool, error)
	ApplyPickup(context.Context, *carerequests.Request, *string, bool) (int64, error)
}

type RequestDecisionEffects interface {
	RegisterDecision(context.Context, *carerequests.Request, carerequests.DecideInput, string, bool) error
	ReloadDecision(context.Context, int64) (*carerequests.ReviewItem, error)
	RecordDecision(context.Context, *carerequests.Request, int64, map[string]any) error
	RecordMarkedDone(context.Context, *carerequests.Request, int64, string) error
	NotifyMarkedDone(context.Context, *carerequests.Request, int64, string) error
}

type RequestDecisionTransaction interface {
	RunInTx(context.Context, func(context.Context) error) error
}
