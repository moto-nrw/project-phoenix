package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

type paperReadStore struct {
	ContractStore
	stage   string
	failure error
}

func (s paperReadStore) Contract(context.Context, int64, int64, bool) (ContractRecord, error) {
	if s.stage == "contract" {
		return ContractRecord{}, s.failure
	}
	return ContractRecord{Summary: carecontract.ContractSummary{ID: 2, SchoolID: 3}}, nil
}

func (s paperReadStore) PaperDocument(context.Context, int64, int64) (carecontract.File, error) {
	if s.stage == "document" {
		return carecontract.File{}, s.failure
	}
	return carecontract.File{Data: []byte("%PDF-signed")}, nil
}

func (s paperReadStore) InsertEvents(context.Context, int64, []Event) error {
	if s.stage == "event" {
		return s.failure
	}
	return nil
}

type paperReadRuntime struct {
	carecontract.Runtime
	t       *testing.T
	stage   string
	failure error
	steps   []string
}

func (r *paperReadRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	require.EqualValues(r.t, 1, organizationID)
	r.steps = append(r.steps, "traeger")
	if r.stage == "begin" {
		return r.failure
	}
	if err := fn(ctx); err != nil {
		return err
	}
	if r.stage == "commit" {
		return r.failure
	}
	r.steps = append(r.steps, "committed")
	return nil
}

func (r *paperReadRuntime) WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	require.EqualValues(r.t, 3, schoolID)
	require.Equal(r.t, []string{"traeger", "committed"}, r.steps, "the Träger transaction must end before the school log")
	r.steps = append(r.steps, "school")
	return fn(ctx)
}

type paperReadAccess struct {
	reads   []carecontract.SchoolAccess
	failure error
}

func (a *paperReadAccess) RecordAccess(_ context.Context, access carecontract.SchoolAccess) error {
	a.reads = append(a.reads, access)
	return a.failure
}

func TestPaperDocumentRequiresSchoolAccessLog(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"success", "begin", "contract", "document", "event", "commit", "access"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("storage unavailable")
			runtime := &paperReadRuntime{t: t, stage: stage, failure: failure}
			access := &paperReadAccess{}
			if stage == "access" {
				access.failure = failure
			}
			s := &Signatures{
				store:     paperReadStore{stage: stage, failure: failure},
				contracts: &Contracts{runtime: runtime, access: access},
			}
			file, err := s.PaperDocument(context.Background(), carecontract.Reviewer{OrganizationID: 1, AccountID: 4}, 2)
			if stage != "success" {
				require.ErrorIs(t, err, failure)
				require.Empty(t, file, "no file leaves without the completed access log")
				if stage != "access" {
					require.Empty(t, access.reads)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, []byte("%PDF-signed"), file.Data)
			require.Equal(t, []string{"traeger", "committed", "school"}, runtime.steps)
			require.Equal(t, []carecontract.SchoolAccess{{
				OrganizationID: 1, AccountID: 4, Data: carecontract.SchoolAccessContracts,
				View: "care_contract_paper_document", Rows: 1, ContractIDs: []int64{2},
			}}, access.reads)
		})
	}
}
