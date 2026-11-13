package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

type signingErrorStore struct {
	ContractStore
	stage   string
	failure error
}

func (s signingErrorStore) SigningLink(context.Context, int64, string) (SigningLinkRecord, error) {
	if s.stage == "link" {
		return SigningLinkRecord{}, s.failure
	}
	return SigningLinkRecord{DocumentID: 1, ExpiresAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
}
func (s signingErrorStore) Contract(context.Context, int64, int64, bool) (ContractRecord, error) {
	if s.stage == "contract" {
		return ContractRecord{}, s.failure
	}
	return ContractRecord{Summary: carecontract.ContractSummary{Status: carecontract.StatusSent}}, nil
}
func (s signingErrorStore) CurrentDocument(context.Context, int64, int64, int64) (StoredDocument, error) {
	if s.stage == "document" {
		return StoredDocument{}, s.failure
	}
	return StoredDocument{ID: 1}, nil
}
func (s signingErrorStore) InsertEvents(context.Context, int64, []Event) error { return s.failure }

func (signingErrorStore) InboxContractAddressed(context.Context, int64, int64, carecontract.InboxReader) (bool, error) {
	return true, nil
}

type signingErrorRuntime struct {
	carecontract.Runtime
	stage   string
	failure error
}

func (r signingErrorRuntime) WithinTraeger(ctx context.Context, _ int64, fn func(context.Context) error) error {
	if r.stage == "begin" {
		return r.failure
	}
	if err := fn(ctx); err != nil {
		return err
	}
	if r.stage == "commit" {
		return r.failure
	}
	return nil
}

type signingErrorSender struct{ failure, reachFailure error }

func (s signingErrorSender) TraegerLetterhead(context.Context, int64) (string, error) {
	return "Träger", s.failure
}

func (s signingErrorSender) WithdrawalReach(context.Context, int64) (carecontract.WithdrawalReach, error) {
	return carecontract.WithdrawalReach{}, s.reachFailure
}

type signingTestTokens struct{ carecontract.SigningTokens }

func (signingTestTokens) Fingerprint(raw string) string { return raw }

func TestSigningLinkPreservesOperationalErrors(t *testing.T) {
	t.Parallel()
	failure := errors.New("storage unavailable")
	for _, stage := range []string{"begin", "link", "contract", "document", "sender", "reach", "event", "commit"} {
		t.Run(stage, func(t *testing.T) {
			store := signingErrorStore{stage: stage}
			if stage != "commit" {
				store.failure = failure
			}
			var senderFailure, reachFailure error
			switch stage {
			case "sender":
				senderFailure = failure
			case "reach":
				reachFailure = failure
			}
			s := &Signatures{store: store, ports: SignaturePorts{
				Runtime: signingErrorRuntime{stage: stage, failure: failure},
				Links:   carecontract.SigningLinks{Tokens: signingTestTokens{}}, Sender: signingErrorSender{failure: senderFailure, reachFailure: reachFailure},
			}, clock: Clock{Now: func() time.Time { return time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC) }}}
			_, err := s.LinkSigning(context.Background(), "1.token")
			require.ErrorIs(t, err, failure)
			require.NotErrorIs(t, err, carecontract.ErrSigningLinkInvalid)
		})
	}
}

func TestPaperSignatureRequiresARecorder(t *testing.T) {
	t.Parallel()
	s := &Signatures{}
	require.ErrorContains(t, s.RecordPaperSignature(context.Background(), carecontract.Reviewer{OrganizationID: 1}, 1, carecontract.PaperInput{}), "reviewer needs a Träger and an account")
}

func TestInboxContractPreservesOperationalErrors(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"contract", "document"} {
		for _, failure := range []error{context.DeadlineExceeded, errors.New("connection unavailable")} {
			t.Run(stage+"/"+failure.Error(), func(t *testing.T) {
				s := &Signatures{store: signingErrorStore{stage: stage, failure: fmt.Errorf("read failed: %w", failure)}}
				_, _, err := s.inboxContract(context.Background(), 1, 1, carecontract.InboxReader{
					AccountID: 1, Email: "parent@example.org",
				}, false)
				require.ErrorIs(t, err, failure)
				require.NotErrorIs(t, err, carecontract.ErrInboxDocumentNotFound)
			})
		}
	}
}

func TestInboxContractHidesMissingRecords(t *testing.T) {
	t.Parallel()
	for stage, failure := range map[string]error{
		"contract": carecontract.ErrContractNotFound,
		"document": carecontract.ErrContractNotSignable,
	} {
		t.Run(stage, func(t *testing.T) {
			s := &Signatures{store: signingErrorStore{stage: stage, failure: fmt.Errorf("read failed: %w", failure)}}
			_, _, err := s.inboxContract(context.Background(), 1, 1, carecontract.InboxReader{
				AccountID: 1, Email: "parent@example.org",
			}, false)
			require.ErrorIs(t, err, carecontract.ErrInboxDocumentNotFound)
		})
	}
}
