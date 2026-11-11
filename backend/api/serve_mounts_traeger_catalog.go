package api

import (
	"context"
	"errors"

	"github.com/go-chi/chi/v5"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	traegerCatalogCompose "github.com/moto-nrw/project-phoenix/workflows/traegercatalog/compose"
	traegerCatalogHTTP "github.com/moto-nrw/project-phoenix/workflows/traegercatalog/http"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog/ports"
)

// mountTraegerCatalogRoutes serves /traeger/catalog: the Träger phases and
// offerings the office keeps, their preview and their rollout into the
// schools with tariff and contract template.
// The copy commands write through the phase administration of Enrollment
// and the care-offering catalog of Care Plan, so a copy passes every check
// of a school's own edit.
func mountTraegerCatalogRoutes(root chi.Router, schoolPhaseAdministration *enrollmentCompose.Phases, schoolCatalog careplan.CareOfferingCatalogCapability, lockRecurrence func(context.Context) error,
	organizations organizationtenancy.Capability, billing parentbilling.CareModulePrices, intake carecontract.Intake,
) error {
	schoolOfferings, err := careplanCompose.NewTraegerOfferingCopies(schoolCatalog, lockRecurrence)
	if err != nil {
		return err
	}
	schoolPhases := schoolPhaseAdministration.TraegerCopies()
	if schoolPhases == nil {
		return errors.New("träger catalog: the phase administration has no copy commands")
	}
	phases, offerings := enrollmentCompose.NewTraegerPhases(), careplanCompose.NewTraegerOfferings()
	catalog, err := traegerCatalogCompose.New(ports.Dependencies{
		Schools: organizations, Phases: phases, Offerings: offerings,
		SchoolPhases: schoolPhases, SchoolOfferings: schoolOfferings,
		Tariffs:           traegerCatalogTariffs(billing),
		ContractTemplates: traegerCatalogContractTemplates(intake),
		Fingerprint:       securityruntime.Fingerprint,
	})
	if err != nil {
		return err
	}
	resource := traegerCatalogHTTP.NewResource(phases, offerings, catalog, common.RequiresPermission(identityaccess.TraegerRightEnrollmentManage))
	root.Mount("/traeger/catalog", resource.Router(ProtectedTraegerRoutes))
	return nil
}

// traegerCatalogTariffs writes a school's module prices in Parent Billing
// and marks its refusals with their registered codes.
func traegerCatalogTariffs(billing parentbilling.CareModulePrices) ports.Tariffs {
	return func(ctx context.Context, organizationID, accountID int64, prices ports.ModulePrices) error {
		return saveTraegerCatalogPrices(ctx, billing, organizationID, accountID, prices)
	}
}

func saveTraegerCatalogPrices(ctx context.Context, billing parentbilling.CareModulePrices, organizationID, accountID int64, prices ports.ModulePrices) error {
	list := parentbilling.CareModulePriceList{
		SchoolID: prices.SchoolID, CareOfferingID: prices.CareOfferingID, ModuleName: prices.ModuleName, ValidFrom: prices.ValidFrom,
	}
	for _, price := range prices.Prices {
		list.Prices = append(list.Prices, parentbilling.CarePrice{
			Weekday: price.Weekday, DaysPerWeek: price.DaysPerWeek, MonthlyCents: price.MonthlyCents,
			SiblingCents: price.SiblingCents, MealExtra: price.MealExtra,
		})
	}
	err := billing.SaveCareModulePrices(ctx, parentbilling.HolidayReviewer{OrganizationID: organizationID, AccountID: accountID}, list)
	var fields validation.Errors
	switch {
	case errors.Is(err, parentbilling.ErrCareTariffMonthBilled):
		return &ports.Refusal{Code: common.CodeParentBillingCareTariffMonthBilled, Err: err}
	case errors.As(err, &fields):
		return &ports.Refusal{Code: common.CodeParentBillingCareTariffInvalid, Err: err}
	}
	return err
}

// traegerCatalogContractTemplates writes a school's contract template in
// Care Contract, which opens its own transactions.
func traegerCatalogContractTemplates(intake carecontract.Intake) ports.ContractTemplates {
	return func(ctx context.Context, organizationID, accountID, schoolID, careOfferingID int64, template careplan.TraegerContractTemplate) error {
		return saveTraegerCatalogTemplate(ctx, intake, organizationID, accountID, schoolID, careOfferingID, template)
	}
}

func saveTraegerCatalogTemplate(ctx context.Context, intake carecontract.Intake, organizationID, accountID, schoolID, careOfferingID int64, template careplan.TraegerContractTemplate) error {
	_, err := intake.SaveTemplate(ctx, carecontract.Reviewer{OrganizationID: organizationID, AccountID: accountID}, schoolID, careOfferingID, carecontract.TemplateInput{
		TermKind: template.TermKind, TermMonths: template.TermMonths, NoticeMonths: template.NoticeMonths, NoticeTo: template.NoticeTo,
		SpecialTermination: template.SpecialTermination, SpecialTerminationNote: template.SpecialTerminationNote,
		StatuteRequired: template.StatuteRequired, StatuteTitle: template.StatuteTitle, StatuteURL: template.StatuteURL,
		ContractPartner: template.ContractPartner,
	})
	var invalid *carecontract.ValidationError
	if errors.As(err, &invalid) {
		return &ports.Refusal{Code: common.CodeCareContractTemplateInvalid, Err: err}
	}
	return err
}

// newTraegerCatalogBilling builds the Parent Billing and Care Contract
// capabilities the rollout writes tariffs and contract templates through,
// over the same ports as their own Träger routes.
func newTraegerCatalogBilling(organizations HolidayOrganizations, settings TraegerBrandingSettings, schoolSettings TraegerSchoolSettings,
	marks owners.MealAttendanceMarks, delivery deliveryRuntime.Capability, outcomes deliveryRuntime.EmailOutcomes, accessLog owners.DataAccessAppender,
	holidays carecontract.SchoolHolidays, parentsURL string,
) (parentbilling.CareModulePrices, carecontract.Intake) {
	contracts := parentBillingCareContracts{contracts: careContractCompose.NewContractVersions()}
	billing, _ := parentBillingCompose.NewCareBilling(contracts, parentBillingCareSchools{organizations: organizations},
		parentBillingSettings{settings: settings}, newParentBillingCareMeals(schoolSettings, marks), newParentBillingLetterheads(settings))
	_, intakePorts := careContractPorts(organizations, settings, schoolSettings, delivery, outcomes, accessLog, holidays, parentsURL)
	return billing, careContractCompose.NewIntake(intakePorts)
}
