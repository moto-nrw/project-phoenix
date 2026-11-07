package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationTenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// The worker's reads for the pushes of a sent contract and of a release for
// parents (#5941): the Betreuungsvertrag says whether a contract still lies
// in the inbox of its address, Organisation & Tenancy reads the release and
// its coverage, and the parent audience projection names the accounts with a
// portal relationship at a school or a group. Every candidate is checked
// again by the parents portal through its own relationships.

// parentContractInboxNotices binds the contract check to the
// Betreuungsvertrag.
type parentContractInboxNotices struct {
	inbox carecontract.InboxNotices
}

// NewParentContractInboxNotices binds the contract check of the push.
func NewParentContractInboxNotices() parentportal.ContractInboxNoticesPort {
	return parentContractInboxNotices{inbox: careContractCompose.NewInboxNotices()}
}

func (n parentContractInboxNotices) ContractNoticeTarget(ctx context.Context, organizationID, contractID int64, email string) (parentportal.ContractNoticeTarget, bool, error) {
	target, found, err := n.inbox.ContractNoticeTarget(ctx, organizationID, contractID, email)
	return parentportal.ContractNoticeTarget(target), found, err
}

// parentReleaseNotices binds the release checks to Organisation & Tenancy.
type parentReleaseNotices struct {
	documents organizationtenancy.ParentTraegerDocumentNotices
}

// NewParentReleaseNotices binds the release checks of the push.
func NewParentReleaseNotices() parentportal.ReleaseNoticesPort {
	return parentReleaseNotices{documents: organizationTenancyCompose.NewSharedTraegerDocuments()}
}

func (n parentReleaseNotices) ActiveParentRelease(ctx context.Context, organizationID, releaseID int64) (parentportal.ReleaseNoticeAudience, bool, error) {
	audience, found, err := n.documents.ActiveParentRelease(ctx, organizationID, releaseID)
	return parentportal.ReleaseNoticeAudience{DocumentID: audience.DocumentID, SchoolID: audience.SchoolID, GroupID: audience.GroupID}, found, err
}

func (n parentReleaseNotices) ReleaseIsNewForParent(ctx context.Context, organizationID, releaseID int64, reader parentportal.SharedDocumentReader) (bool, error) {
	return n.documents.ReleaseIsNewForParent(ctx, organizationID, releaseID, organizationtenancy.TraegerDocumentParent(reader))
}

// parentReleaseSchools names the schools of a Träger for a release to
// every school.
type parentReleaseSchools struct {
	schools organizationtenancy.SchoolListQuery
}

// NewParentReleaseSchools binds the schools of a release's push.
func NewParentReleaseSchools(schools organizationtenancy.SchoolListQuery) parentportal.ReleaseSchoolsPort {
	return parentReleaseSchools{schools: schools}
}

func (r parentReleaseSchools) TraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error) {
	schools, err := r.schools.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		if !school.IsDeleted() {
			ids = append(ids, school.ID)
		}
	}
	return ids, nil
}

// parentReleaseRecipients names the accounts a release may reach at a
// school, through the parent audience projection.
type parentReleaseRecipients struct {
	audience communication.ParentGroupAudienceReads
}

// NewParentReleaseRecipients binds the recipients of a release's push.
func NewParentReleaseRecipients(audience communication.ParentGroupAudienceReads) parentportal.ReleaseRecipientsPort {
	return parentReleaseRecipients{audience: audience}
}

// GuardianAccounts lists, with the rules of a parent group thread, the
// accounts whose relationship to a child of the school, of the group when
// set, grants parent_portal.access.
func (r parentReleaseRecipients) GuardianAccounts(ctx context.Context, schoolID int64, groupID *int64) ([]int64, error) {
	target := communication.ParentGroupTarget{Type: "school_all"}
	if groupID != nil {
		target = communication.ParentGroupTarget{Type: "group", RefID: groupID}
	}
	children, err := r.audience.GroupAudience(ctx, schoolID, target)
	if err != nil {
		return nil, err
	}
	var accounts []int64
	for _, child := range children {
		accounts = append(accounts, child.GuardianAccountIDs...)
	}
	return accounts, nil
}
