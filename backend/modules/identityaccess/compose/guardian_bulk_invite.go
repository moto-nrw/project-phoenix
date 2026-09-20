package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (e engine) BulkInviteToStudents(ctx context.Context, req identityaccess.BulkInviteRequest) (*identityaccess.BulkInviteResult, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	result, err := e.lifecycle.BulkInviteToStudents(e.attach(ctx), domain.BulkInviteRequest(req))
	if err != nil {
		return nil, lifecycleError(err)
	}
	public := identityaccess.BulkInviteResult{
		DryRun: result.DryRun, Invited: result.Invited, LinkedExistingAccount: result.LinkedExistingAccount,
		Resent: result.Resent, SkippedActive: result.SkippedActive, SkippedOpen: result.SkippedOpen,
		SkippedRestricted: result.SkippedRestricted, Problems: make([]identityaccess.BulkInviteProblem, 0, len(result.Problems)),
	}
	for _, problem := range result.Problems {
		public.Problems = append(public.Problems, identityaccess.BulkInviteProblem(problem))
	}
	return &public, nil
}
