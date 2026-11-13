//go:build testhooks

package owners

import (
	"context"
	"sync"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-ogs/project-phoenix/modules/carecontract/compose"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// CareContractTestHolidays answers the same statutory holidays for every
// school a test composition asks; empty, only Saturday and Sunday move a
// withdrawal period (#6350).
type CareContractTestHolidays map[calendar.Date]bool

func (h CareContractTestHolidays) SchoolHolidayDates(_ context.Context, schoolIDs []int64, _, _ calendar.Date) (map[int64]map[calendar.Date]bool, error) {
	result := make(map[int64]map[calendar.Date]bool, len(schoolIDs))
	for _, schoolID := range schoolIDs {
		result[schoolID] = h
	}
	return result, nil
}

// CareContractTestMails records the mails of the Betreuungsvertrag a test
// composition enqueues, instead of Delivery's outbox.
type CareContractTestMails struct {
	mu    sync.Mutex
	mails []carecontract.ContractMail
}

func (m *CareContractTestMails) EnqueueContractMail(_ context.Context, mail carecontract.ContractMail) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mails = append(m.mails, mail)
	return int64(len(m.mails)), nil
}

func (m *CareContractTestMails) ContractMailStates(context.Context, int64, int64) ([]carecontract.MailState, error) {
	return nil, nil
}

// ShareMailBlockLimit shares nothing; the test mails read no provider.
func (m *CareContractTestMails) ShareMailBlockLimit(ctx context.Context) context.Context { return ctx }

// ContractMailBlock holds no address back.
func (m *CareContractTestMails) ContractMailBlock(context.Context, int64, string, string) (string, error) {
	return "", nil
}

// Purposes lists the purposes of the recorded mails.
func (m *CareContractTestMails) Purposes() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	purposes := make([]string, len(m.mails))
	for i, mail := range m.mails {
		purposes[i] = mail.Purpose
	}
	return purposes
}

// careContractTestSender names every Träger "Träger".
type careContractTestSender struct{}

func (careContractTestSender) TraegerLetterhead(context.Context, int64) (string, error) {
	return "Träger e.V.\nMusterweg 1\n12345 Musterstadt", nil
}

func (careContractTestSender) WithdrawalReach(context.Context, int64) (carecontract.WithdrawalReach, error) {
	return carecontract.WithdrawalReach{ContactPhone: "0123 456", ContactEmail: "vertrag@example.org"}, nil
}

// NewParentContractSigningForTests composes the signature in the parents'
// inbox (#4679) over the real ledger and store, with the mails recorded.
func NewParentContractSigningForTests(mails *CareContractTestMails) carecontract.InboxSigning {
	return careContractCompose.NewInboxSigning(careContractCompose.SignaturePorts{
		Ledger: NewCareContractSignatures(), Links: CareContractLinks("http://parents.localhost:3000"),
		Sender: careContractTestSender{}, Post: mails, Runtime: CareContractRuntime{}, Holidays: CareContractTestHolidays{}, Families: NewCareContractFamilyNumbers(),
	})
}

// RecordCareContractSendForTests stores the signing link of a contract a
// test wrote directly, as a send would, in the caller's Träger transaction.
func RecordCareContractSendForTests(ctx context.Context, organizationID, contractID int64) error {
	return careContractCompose.SendLinkForTests(ctx, CareContractLinks("http://parents.localhost:3000"), organizationID, contractID)
}
