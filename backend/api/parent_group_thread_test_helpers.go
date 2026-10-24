//go:build testhooks

package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// The group threads (#4784) for the parent portal suites, which may not
// import Communication: the guardian port, and the OGS side as an admin
// staff member writing to a class.

// NewParentGroupThreadTestPort binds the guardian side of the group threads.
func NewParentGroupThreadTestPort(db transactionruntime.Database) parentService.GroupThreads {
	return owners.BindParentGroupThreads(parentStore.NewGroupThreadPortal(db.Pool()))
}

// CreateParentGroupClassThreadForTest writes a group thread to a class as the
// staff account, inside the caller's school transaction.
func CreateParentGroupClassThreadForTest(ctx context.Context, db transactionruntime.Database, staffAccountID int64, class, title, body string) (int64, error) {
	detail, err := parentGroupThreadsForTest(db, staffAccountID).CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
		Target: communication.ParentGroupTarget{Type: communication.ParentGroupTargetClass, RefText: &class, Label: "Klasse " + class},
		Title:  title, Body: body,
	})
	if err != nil {
		return 0, err
	}
	return detail.Thread.ID, nil
}

// CloseParentGroupThreadForTest closes a group thread as the staff account,
// inside the caller's school transaction.
func CloseParentGroupThreadForTest(ctx context.Context, db transactionruntime.Database, staffAccountID, threadID int64) error {
	_, err := parentGroupThreadsForTest(db, staffAccountID).CloseParentGroupThread(ctx, threadID)
	return err
}

func parentGroupThreadsForTest(db transactionruntime.Database, staffAccountID int64) communication.ParentGroupThreadCapability {
	messages := NewParentMessagingSuiteRepositories(db)
	_, groups := communicationCompose.NewParentMessaging(communicationCompose.ParentMessagingConfig{
		ThreadRepo: messages.ParentMessageThread, MessageRepo: parentStore.NewParentMessageRepository(db.Pool()),
		ReadRepo: messages.ParentMessageRead, UserContext: owners.StaffStudentReads{}, Session: groupThreadTestSession(staffAccountID),
		Settings: groupThreadTestSettings{}, DB: db.Pool(), Logger: slog.Default(),
		GroupAudience: owners.NewParentGroupAudience(db),
	}, owners.CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	return groups
}

// groupThreadTestSession is a school admin who reads every child.
type groupThreadTestSession int64

func (s groupThreadTestSession) AccountID(context.Context) int64 { return int64(s) }
func (groupThreadTestSession) Permissions(context.Context) []string {
	return []string{"admin:*"}
}

type groupThreadTestSettings struct{}

func (groupThreadTestSettings) ResolveBool(context.Context, string) (bool, error) { return true, nil }
func (groupThreadTestSettings) ParentMessageStaffNameVisible(context.Context) (bool, error) {
	return false, nil
}
