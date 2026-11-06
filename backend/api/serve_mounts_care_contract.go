package api

import (
	"context"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	careContractHTTP "github.com/moto-nrw/project-phoenix/modules/carecontract/http"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationTenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// mountCareContractRoutes serves the Betreuungsvertrag (ADR 0088, ADR 0153,
// #4678) behind the right "Verträge verwalten": the contracts at
// /traeger/contracts and the templates of the offerings at
// /traeger/contract-templates.
func mountCareContractRoutes(root chi.Router, organizations HolidayOrganizations, settings TraegerBrandingSettings,
	delivery deliveryRuntime.Capability, accessLog owners.DataAccessAppender, parentsURL string) {
	resource := NewCareContractHTTP(organizations, settings, delivery, accessLog, parentsURL)
	root.Mount("/traeger/contracts", resource.ContractRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/contract-templates", resource.TemplateRouter(ProtectedTraegerRoutes))
	// The one-time link of a contract mail (#4679), without an account.
	signing := careContractCompose.NewSigning(careContractSignaturePorts(organizations, settings, delivery, parentsURL))
	root.Mount("/public/care-contract-signing", careContractHTTP.NewSigningResource(signing).PublicRouter())
}

// careContractSignaturePorts binds the signature of a contract (#4679): the
// Audit Platform's ledger at the Träger, the one-time link tokens with the
// base of the parents portal, the name and letterhead of the Träger and
// Delivery's outbox.
func careContractSignaturePorts(organizations HolidayOrganizations, settings TraegerBrandingSettings,
	delivery deliveryRuntime.Capability, parentsURL string) careContractCompose.SignaturePorts {
	return careContractCompose.SignaturePorts{
		Ledger: owners.NewCareContractSignatures(), Links: owners.CareContractLinks(parentsURL),
		Sender: owners.CareContractSender{Settings: settings},
		Post:   owners.CareContractPost{Delivery: delivery}, Runtime: owners.CareContractRuntime{},
	}
}

// NewCareContractHTTP binds the Betreuungsvertrag's ports: the schools of
// the Träger to Organisation & Tenancy, the accepted applications to
// Enrollment and the offerings and bookings to Care Plan in each school's
// tenant transaction, the letterhead and the contract template to the
// master data and the branding of the Träger, and the mail to Delivery.
func NewCareContractHTTP(organizations HolidayOrganizations, settings TraegerBrandingSettings,
	delivery deliveryRuntime.Capability, accessLog owners.DataAccessAppender, parentsURL string) *careContractHTTP.Resource {
	letterheads := careContractLetterheads{masterData: organizationTenancyCompose.NewTraegerMasterData(),
		templates: organizationTenancyCompose.NewTraegerDocumentTemplates(), settings: settings}
	contractPorts := careContractCompose.ContractPorts{
		Letterheads: letterheads, Post: owners.CareContractPost{Delivery: delivery},
		Runtime: owners.CareContractRuntime{}, Access: careContractAccessLog{log: accessLog},
		Ledger: owners.NewCareContractSignatures(), Links: owners.CareContractLinks(parentsURL),
	}
	contracts := careContractCompose.NewContracts(contractPorts)
	paper := careContractCompose.NewPaperSignatures(contractPorts, careContractSignaturePorts(organizations, settings, delivery, parentsURL))
	intake := careContractCompose.NewIntake(careContractCompose.IntakePorts{
		Schools: careContractSchools{organizations: organizations}, Applications: careContractApplications{applications: enrollmentCompose.NewContractApplications()},
		Bookings: careContractBookings{offerings: carePlanCompose.NewContractOfferings(), bookings: carePlanCompose.NewOfferingBookings()},
		Access:   careContractAccessLog{log: accessLog}, Runtime: owners.CareContractRuntime{},
	})
	return careContractHTTP.NewResource(contracts, intake, paper, common.RequiresPermission(identityaccess.TraegerRightContractsManage))
}

// careContractSchools lists the undeleted schools of the Träger.
type careContractSchools struct {
	organizations HolidayOrganizations
}

func (s careContractSchools) TraegerSchools(ctx context.Context, organizationID int64) ([]carecontract.School, error) {
	schools, err := s.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	result := make([]carecontract.School, 0, len(schools))
	for _, school := range schools {
		if !school.IsDeleted() {
			result = append(result, carecontract.School{ID: school.ID, Name: school.Name})
		}
	}
	slices.SortFunc(result, func(a, b carecontract.School) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return int(a.ID - b.ID)
	})
	return result, nil
}

// careContractApplications reads the accepted applications of a school
// from Enrollment in its tenant transaction.
type careContractApplications struct {
	applications enrollment.ContractApplications
}

func (a careContractApplications) AcceptedApplications(ctx context.Context, requestChildIDs []int64) ([]carecontract.Application, error) {
	found, err := a.applications.AcceptedContractApplications(ctx, requestChildIDs)
	if err != nil {
		return nil, err
	}
	result := make([]carecontract.Application, len(found))
	for i, application := range found {
		result[i] = carecontract.Application{
			RequestChildID: application.RequestChildID, StudentID: application.StudentID,
			ChildFirstName: application.ChildFirstName, ChildLastName: application.ChildLastName,
			ChildBirthDate: application.ChildBirthDate, GuardianFirstName: application.GuardianFirstName,
			GuardianLastName: application.GuardianLastName, GuardianEmail: application.GuardianEmail,
			GuardianAccountID: application.GuardianAccountID, PhaseName: application.PhaseName,
			ServiceStartDate: application.ServiceStartDate, ActivateOn: application.ActivateOn,
		}
	}
	return result, nil
}

func (a careContractApplications) NamePhases(ctx context.Context, offerings []carecontract.Offering) ([]carecontract.Offering, error) {
	phaseIDs := []int64{}
	for _, offering := range offerings {
		if !slices.Contains(phaseIDs, offering.PhaseID) {
			phaseIDs = append(phaseIDs, offering.PhaseID)
		}
	}
	names, err := a.applications.ContractPhases(ctx, phaseIDs)
	if err != nil {
		return nil, err
	}
	named := slices.Clone(offerings)
	for i := range named {
		phase := names[named[i].PhaseID]
		named[i].PhaseName = phase.Name
		named[i].ServiceStart, named[i].ServiceEnd = phase.ServiceStart, phase.ServiceEnd
	}
	return named, nil
}

// careContractBookings reads the bookings and offerings of a school from
// Care Plan in its tenant transaction.
type careContractBookings struct {
	offerings careplan.ContractOfferings
	bookings  *careplan.OfferingBookings
}

func (b careContractBookings) Bookings(ctx context.Context, requestChildIDs []int64) (map[int64][]carecontract.Booking, error) {
	history, err := b.bookings.CareOfferingBookingHistory(ctx, requestChildIDs)
	if err != nil {
		return nil, err
	}
	byChild := map[int64][]carecontract.Booking{}
	for _, booking := range history {
		byChild[booking.RequestChildID] = append(byChild[booking.RequestChildID], carecontract.Booking{
			CareOfferingID: booking.CareOfferingID, Days: booking.EffectiveSelectedDays(),
			ValidFrom: careContractDate(booking.ValidFrom), ValidUntil: careContractDate(booking.ValidUntil),
		})
	}
	return byChild, nil
}

func careContractDate(date *careplan.Date) *calendar.Date {
	if date == nil {
		return nil
	}
	value := calendar.Date(*date)
	return &value
}

func (b careContractBookings) Offerings(ctx context.Context, ids []int64) ([]carecontract.Offering, error) {
	found, err := b.offerings.ContractOfferings(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carecontract.Offering, len(found))
	for i, offering := range found {
		result[i] = carecontract.Offering{
			ID: offering.ID, PhaseID: offering.PhaseID, Name: offering.Name, PriceCents: offering.PriceCents, Active: offering.IsActive,
			EndTimes: offering.PickupTimes, DaysOfWeekMode: offering.DaysOfWeekMode, AvailableDays: offering.AvailableDays,
		}
	}
	return result, nil
}

// careContractAccessLog writes a read of the Träger-Büro into the access
// log of a school (ADR 0084 section 7).
type careContractAccessLog struct {
	log owners.DataAccessAppender
}

func (l careContractAccessLog) RecordAccess(ctx context.Context, access carecontract.SchoolAccess) error {
	now := time.Now()
	resource := auditModels.ResourceTypeCareContractApplications
	metadata := map[string]any{
		"channel": "traeger_portal", "organization_id": access.OrganizationID, "view": access.View, "rows": access.Rows,
	}
	if access.Data == carecontract.SchoolAccessContracts {
		resource = auditModels.ResourceTypeCareContracts
		metadata["contract_ids"] = access.ContractIDs
	}
	return l.log.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: access.AccountID, ActorRole: traegerAccessActorRole,
		ResourceType: resource, RangeStart: now, RangeEnd: now, Metadata: metadata,
	})
}

// careContractLetterheads binds the letterhead of the contract to the
// master data of Organisation & Tenancy, the branding settings of the
// Träger and its template of the kind "Vertrag" (#4677, ADR 0152).
type careContractLetterheads struct {
	masterData organizationtenancy.TraegerMasterData
	templates  organizationtenancy.TraegerDocumentTemplates
	settings   TraegerBrandingSettings
}

func (b careContractLetterheads) Letterhead(ctx context.Context, reviewer carecontract.Reviewer) (carecontract.Letterhead, error) {
	profile, err := b.masterData.Profile(ctx, reviewer.OrganizationID, reviewer.AccountID)
	if err != nil {
		return carecontract.Letterhead{}, err
	}
	texts, err := b.settings.TraegerTexts(ctx, reviewer.OrganizationID, []string{
		configModels.KeyBrandingPrimaryColor, configModels.KeyBrandingLetterheadText, configModels.KeyBrandingFooterText,
	})
	if err != nil {
		return carecontract.Letterhead{}, err
	}
	template, err := b.templates.Current(ctx, reviewer.OrganizationID, templates.KindContract)
	if err != nil {
		return carecontract.Letterhead{}, err
	}
	head := carecontract.Letterhead{
		AccentColor: texts[configModels.KeyBrandingPrimaryColor], SenderText: texts[configModels.KeyBrandingLetterheadText],
		FooterText: texts[configModels.KeyBrandingFooterText], ContactName: profile.Contact.Name,
		ContactPhone: profile.Contact.Phone, ContactEmail: profile.Contact.Email,
		Template: carecontract.LetterTemplate{
			Version: template.Version, Subject: template.Subject, Salutation: template.Salutation,
			Body: template.Body, Closing: template.Closing,
		},
	}
	if profile.Logo == nil {
		return head, nil
	}
	logo, err := b.masterData.LogoImage(ctx, reviewer.OrganizationID)
	if err != nil {
		return carecontract.Letterhead{}, err
	}
	head.Logo = logo.Data
	return head, nil
}
