package api

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	auditModels "github.com/moto-ogs/project-phoenix/modules/auditlog/records"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	organizationTenancyCompose "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/compose"

	organizationTenancyTraeger "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/inbound/traeger"
	"github.com/moto-ogs/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-ogs/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// traegerDocumentGroupLimit bounds the groups one school offers a release.
const traegerDocumentGroupLimit = 500

// mountTraegerDocumentRoutes serves the documents a Träger shares (#4692):
// the Träger-Büro's routes at /traeger/documents and the school's at
// /api/traeger-documents. The parents read them through the parents portal.
func mountTraegerDocumentRoutes(root, tenantRouter chi.Router, db transactionruntime.Database, organizations HolidayOrganizations, accessLog owners.DataAccessAppender, ledger auditModels.Command, delivery owners.DeferredPushOutbox) error {
	traegerDocuments, err := NewTraegerDocumentsHTTP(db, organizations, accessLog, ledger, delivery)
	if err != nil {
		return err
	}
	root.Mount("/traeger/documents", traegerDocuments.TraegerRouter(ProtectedTraegerRoutes))
	tenantRouter.Mount("/traeger-documents", NewSchoolTraegerDocumentsHTTP().Router(ProtectedTenantRoutes))
	return nil
}

// NewTraegerDocumentsHTTP binds the Träger-Büro's documents: the schools to
// Organisation & Tenancy, the groups of a school to School Structure in
// that school's tenant transaction and the push of a release for parents to
// Delivery's outbox (#5941).
func NewTraegerDocumentsHTTP(db transactionruntime.Database, organizations HolidayOrganizations, accessLog owners.DataAccessAppender, ledger auditModels.Command, delivery owners.DeferredPushOutbox) (*organizationTenancyTraeger.DocumentsResource, error) {
	groups, err := schoolStructureCompose.New(schoolStructureCompose.Dependencies{DB: db.Pool(), Observe: func(schoolStructureCompose.Observation) {}})
	if err != nil {
		return nil, fmt.Errorf("traeger documents: school groups: %w", err)
	}
	documents := organizationTenancyCompose.NewTraegerDocuments(organizationtenancy.TraegerDocumentPorts{
		Schools:   traegerDocumentSchools{organizations: organizations},
		Groups:    traegerDocumentGroups{groups: groups},
		Runtime:   traegerDocumentRuntime{},
		Access:    traegerDocumentAccessLog{log: accessLog},
		Downloads: traegerDocumentDownloadLedger{command: ledger},
		Notices:   owners.TraegerDocumentReleaseNotices{Delivery: delivery},
	})
	return organizationTenancyTraeger.NewDocumentsResource(documents,
		common.RequiresPermission(identityaccess.TraegerRightOverviewRead), common.RequiresPermission(identityaccess.TraegerRightDocumentsManage)), nil
}

// NewSchoolTraegerDocumentsHTTP binds the school's read of the documents
// its Träger released, behind the right to manage the school's files.
func NewSchoolTraegerDocumentsHTTP() *organizationTenancyTraeger.SchoolDocumentsResource {
	return organizationTenancyTraeger.NewSchoolDocumentsResource(organizationTenancyCompose.NewSharedTraegerDocuments(),
		traegerDocumentRuntime{}, common.RequiresPermission(permissions.FilesManage))
}

// traegerDocumentSchools lists the undeleted schools of the Träger by name.
type traegerDocumentSchools struct {
	organizations HolidayOrganizations
}

func (s traegerDocumentSchools) TraegerSchools(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerDocumentSchool, error) {
	schools, err := s.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	result := make([]organizationtenancy.TraegerDocumentSchool, 0, len(schools))
	for _, school := range schools {
		if !school.IsDeleted() {
			result = append(result, organizationtenancy.TraegerDocumentSchool{ID: school.ID, Name: school.Name})
		}
	}
	slices.SortFunc(result, func(a, b organizationtenancy.TraegerDocumentSchool) int {
		if order := strings.Compare(a.Name, b.Name); order != 0 {
			return order
		}
		return int(a.ID - b.ID)
	})
	return result, nil
}

// traegerDocumentGroups reads the groups of the school of the tenant
// transaction from School Structure.
type traegerDocumentGroups struct {
	groups schoolstructure.GroupListing
}

func (g traegerDocumentGroups) SchoolGroups(ctx context.Context) ([]organizationtenancy.TraegerDocumentGroup, error) {
	groups, err := g.groups.ListGroups(ctx, traegerDocumentGroupLimit)
	if err != nil {
		return nil, err
	}
	result := make([]organizationtenancy.TraegerDocumentGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, organizationtenancy.TraegerDocumentGroup{ID: group.ID, Name: group.Name})
	}
	return result, nil
}

// traegerDocumentRuntime opens a school's tenant transaction or the Träger
// transaction, never both at once (ADR 0105).
type traegerDocumentRuntime struct{}

func (traegerDocumentRuntime) WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	school, err := tenant.NewTenantID(schoolID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, school, fn)
}

func (traegerDocumentRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(ctx, organizationID, fn)
}

// traegerDocumentAccessLog writes a download of the Träger-Büro into the
// access log of a school (ADR 0084 section 7).
type traegerDocumentAccessLog struct {
	log owners.DataAccessAppender
}

func (l traegerDocumentAccessLog) RecordAccess(ctx context.Context, access organizationtenancy.TraegerDocumentAccess) error {
	now := time.Now()
	return l.log.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: access.AccountID, ActorRole: traegerAccessActorRole,
		ResourceType: auditModels.ResourceTypeTraegerDocumentDownload, RangeStart: now, RangeEnd: now,
		Metadata: map[string]any{
			"channel": "traeger_portal", "organization_id": access.OrganizationID, "document_id": access.DocumentID,
		},
	})
}

// traegerDocumentDownloadLedger appends a download of the Träger-Büro to the
// Träger's download ledger of the Audit Platform in the caller's Träger
// transaction (ADR 0157).
type traegerDocumentDownloadLedger struct {
	command auditModels.Command
}

func (l traegerDocumentDownloadLedger) RecordDownload(ctx context.Context, download organizationtenancy.TraegerDocumentDownload) error {
	return l.command.Append(ctx, &auditModels.TraegerDocumentDownload{
		OrganizationID: download.OrganizationID, DocumentID: download.DocumentID, ActorAccountID: download.AccountID,
		ClientIP: download.ClientIP, UserAgent: download.UserAgent,
	})
}
