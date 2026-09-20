package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

type RequestCorrectionRecords interface {
	Find(context.Context, int64, bool) (carerequests.Request, error)
	Redecide(context.Context, int64, string, *string, int64, bool) error
	FindException(context.Context, int64) (*careplan.PickupException, error)
	DeleteException(context.Context, int64) error
}

type RequestCorrectionEvent struct {
	Type    string
	Payload map[string]any
}

type RequestCorrectionEffects interface {
	CorrectionHistory(context.Context, *carerequests.Request) ([]RequestCorrectionEvent, error)
	RecordCorrection(context.Context, *carerequests.Request, int64, map[string]any) error
	NotifyCorrection(context.Context, *carerequests.Request, int64, bool, string) error
}

type RequestCorrectionPeople interface {
	LockStudent(context.Context, int64) (ReviewStudent, error)
	CanReview(context.Context, ReviewStudent) (bool, error)
}

type RequestCorrectionPlans interface {
	ApplyPickup(context.Context, *carerequests.Request, *string, bool) (int64, error)
}
