package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The revenue accounts and cost centers of a Träger (#4821) and how a
// position finds its own: an Angebot's revenue account before its
// Leistung's before the standard of the DATEV settings; a cost center from
// its day until the next one of its scope. The release copies the result
// onto each position (bookingResolver.care, .meal, .holiday), so a later
// change reaches only positions issued after it.

// BookingAssignmentReader reads the revenue accounts and cost centers of a
// Träger for the release and the export (#4821).
type BookingAssignmentReader interface {
	BookingAssignments(ctx context.Context, organizationID int64) (parentbilling.BookingAssignments, error)
}

// BookingAccountStore persists the revenue accounts and cost centers. Every
// method runs in the Träger transaction of organizationID.
type BookingAccountStore interface {
	BookingAssignmentReader
	// BookingOfferings lists the Angebote of the Träger's prices.
	BookingOfferings(ctx context.Context, organizationID int64) ([]parentbilling.BookingOffering, error)
	SetRevenueAccount(ctx context.Context, organizationID, accountID int64, scope string, scopeID int64, account string) error
	DeleteRevenueAccount(ctx context.Context, organizationID int64, scope string, scopeID int64) error
	// InsertCostCenter is ErrCostCenterDateTaken for a second row of a
	// scope on one day.
	InsertCostCenter(ctx context.Context, organizationID, accountID int64, center parentbilling.CostCenter) error
	// DeleteCostCenter is ErrCostCenterNotFound for a row of another
	// Träger or none.
	DeleteCostCenter(ctx context.Context, organizationID, id int64) error
}

// validRevenueAccount is the save rule of a revenue account: the rule of a
// DATEV Sachkonto at its longest length (digits, not zero, at most 8). That
// it fits the Sachkontenlänge of the Träger is a rule of two values; the
// DATEV file names it.
func validRevenueAccount(account string) bool {
	return datevGeneralAccount(datevMaxAccountLength)(account)
}

// validCostCenter is the save rule of a cost center: the DATEV rule of
// KOST1 and KOST2, without spaces at its ends.
func validCostCenter(code string) bool {
	return code == strings.TrimSpace(code) && datevTextRules.costCenter1.allows(code)
}

// bookingResolver resolves where a position books.
type bookingResolver struct {
	standard string
	revenue  map[bookingScope]string
	centers  map[bookingScope][]parentbilling.CostCenter
}

type bookingScope struct {
	scope string
	id    int64
}

// newBookingResolver builds the resolver of the assignments and the
// standard account of the DATEV settings.
func newBookingResolver(assignments parentbilling.BookingAssignments, standard string) bookingResolver {
	r := bookingResolver{standard: standard, revenue: map[bookingScope]string{}, centers: map[bookingScope][]parentbilling.CostCenter{}}
	for _, account := range assignments.RevenueAccounts {
		r.revenue[bookingScope{account.Scope, account.ScopeID}] = account.Account
	}
	for _, center := range assignments.CostCenters {
		key := bookingScope{center.Scope, center.ScopeID}
		r.centers[key] = append(r.centers[key], center)
	}
	for key := range r.centers {
		slices.SortFunc(r.centers[key], func(a, b parentbilling.CostCenter) int { return a.ValidFrom.Compare(b.ValidFrom) })
	}
	return r
}

// account is the first revenue account of the scopes, the standard without
// one.
func (r bookingResolver) account(scopes ...bookingScope) string {
	for _, scope := range scopes {
		if account := r.revenue[scope]; account != "" {
			return account
		}
	}
	return r.standard
}

// costCenter is the code of the scope on day: the latest row from day or
// before, empty without one (invariant 2).
func (r bookingResolver) costCenter(scope bookingScope, day calendar.Date) string {
	code := ""
	for _, center := range r.centers[scope] {
		if center.ValidFrom.After(day) {
			break
		}
		code = center.Code
	}
	return code
}

// care is where a position of the monthly contributions of an Angebot at a
// school books from day on.
func (r bookingResolver) care(schoolID, offeringID int64, day calendar.Date) parentbilling.PositionAccounts {
	offering := bookingScope{parentbilling.BookingScopeOffering, offeringID}
	return parentbilling.PositionAccounts{
		RevenueAccount: r.account(offering, bookingScope{parentbilling.BookingScopeCare, 0}),
		CostCenter:     r.costCenter(bookingScope{parentbilling.BookingScopeSchool, schoolID}, day),
		CostCenter2:    r.costCenter(offering, day),
		Frozen:         true,
	}
}

// meal is where a Mensa month of a school books.
func (r bookingResolver) meal(schoolID int64, month calendar.Date) parentbilling.PositionAccounts {
	return parentbilling.PositionAccounts{
		RevenueAccount: r.account(bookingScope{parentbilling.BookingScopeMeals, 0}),
		CostCenter:     r.costCenter(bookingScope{parentbilling.BookingScopeSchool, schoolID}, month),
		Frozen:         true,
	}
}

// holiday is where a holiday position books from day on on the KOST1 of
// the holiday care: a position issued before #4821 at export, and the
// fallback of holidayWeek.
func (r bookingResolver) holiday(day calendar.Date) parentbilling.PositionAccounts {
	return parentbilling.PositionAccounts{
		RevenueAccount: r.account(bookingScope{parentbilling.BookingScopeHoliday, 0}),
		CostCenter:     r.costCenter(bookingScope{parentbilling.BookingScopeHoliday, 0}, day),
		Frozen:         true,
	}
}

// holidayWeek is where a position of a holiday week books (invariant 5):
// on the KOST1 of its host school on the first day of the week, else on
// the holiday care's with the reason. hostSchoolID is zero without a host
// school.
func (r bookingResolver) holidayWeek(hostSchoolID int64, day calendar.Date) (parentbilling.PositionAccounts, string) {
	accounts := r.holiday(day)
	if hostSchoolID == 0 {
		return accounts, parentbilling.CostCenterFallbackNoHostSchool
	}
	code := r.costCenter(bookingScope{parentbilling.BookingScopeSchool, hostSchoolID}, day)
	if code == "" {
		return accounts, parentbilling.CostCenterFallbackNoSchoolCostCenter
	}
	accounts.CostCenter = code
	return accounts, ""
}

// bookingResolverOf reads the assignments and the standard account of the
// Träger in its transaction.
func bookingResolverOf(ctx context.Context, store BookingAssignmentReader, settings parentbilling.DatevBillingSettings, organizationID int64,
) (bookingResolver, error) {
	assignments, err := store.BookingAssignments(ctx, organizationID)
	if err != nil {
		return bookingResolver{}, err
	}
	datev, err := settings.DatevSettings(ctx, organizationID)
	if err != nil {
		return bookingResolver{}, fmt.Errorf("parent billing: read the standard revenue account: %w", err)
	}
	return newBookingResolver(assignments, strings.TrimSpace(datev.RevenueAccount.Value)), nil
}

// freezeCareAccounts copies where each position of the monthly invoices
// books onto it (invariant 1): its first service day picks the cost
// centers.
func freezeCareAccounts(resolver bookingResolver, invoices []parentbilling.CareInvoice) {
	for i := range invoices {
		invoice := &invoices[i]
		for j := range invoice.Positions {
			position := &invoice.Positions[j]
			position.Accounts = resolver.care(invoice.SchoolID, position.CareOfferingID, position.ServiceFrom)
		}
		for j := range invoice.MealPositions {
			meal := &invoice.MealPositions[j]
			meal.Accounts = resolver.meal(meal.SchoolID, meal.MealMonth)
		}
	}
}

// freezeHolidayAccounts copies where each position of the holiday
// documents books onto it (invariants 1 and 5). A base line books on the
// host school of its week, found by its ID or else by its days; every
// other line of a child (discount, exemption, cost takeover, correction)
// books like the child's first week, so it relieves the same KOST1. A
// child without a week books on the holiday care from the first day of
// the document.
func freezeHolidayAccounts(ctx context.Context, store BookingAssignmentReader, settings parentbilling.DatevBillingSettings,
	organizationID int64, weeks []parentbilling.HolidayWeekHost, documents []parentbilling.BillingDocument,
) error {
	resolver, err := bookingResolverOf(ctx, store, settings, organizationID)
	if err != nil {
		return err
	}
	hosts := newHolidayHosts(weeks)
	for i := range documents {
		for j := range documents[i].Children {
			freezeHolidayChild(resolver, hosts, documents[i].ServiceFrom, &documents[i].Children[j])
		}
	}
	return nil
}

func freezeHolidayChild(resolver bookingResolver, hosts holidayHosts, serviceFrom calendar.Date, child *parentbilling.BillingChild) {
	other, reason := resolver.holiday(serviceFrom), parentbilling.CostCenterFallbackNoHostSchool
	first := true
	for i := range child.Positions {
		position := &child.Positions[i]
		if position.WeekStartsOn == nil {
			continue
		}
		position.Accounts, position.CostCenterFallback = resolver.holidayWeek(hosts.school(*position), *position.WeekStartsOn)
		if first {
			other, reason, first = position.Accounts, position.CostCenterFallback, false
		}
	}
	for i := range child.Positions {
		if position := &child.Positions[i]; position.WeekStartsOn == nil {
			position.Accounts, position.CostCenterFallback = other, reason
		}
	}
}

// holidayHosts finds the host school of the week of a position.
type holidayHosts struct {
	byID   map[int64]int64
	byDays map[[2]calendar.Date][]int64
}

func newHolidayHosts(weeks []parentbilling.HolidayWeekHost) holidayHosts {
	hosts := holidayHosts{byID: map[int64]int64{}, byDays: map[[2]calendar.Date][]int64{}}
	for _, week := range weeks {
		hosts.byID[week.WeekID] = week.HostSchoolID
		days := [2]calendar.Date{week.StartsOn, week.EndsOn}
		if !slices.Contains(hosts.byDays[days], week.HostSchoolID) {
			hosts.byDays[days] = append(hosts.byDays[days], week.HostSchoolID)
		}
	}
	return hosts
}

// school is the host school of the position's week, zero when no week of
// the period has its ID, or, without one, when no week or weeks of more
// than one school have its days.
func (h holidayHosts) school(position parentbilling.BillingPosition) int64 {
	if position.WeekID != 0 {
		return h.byID[position.WeekID]
	}
	if position.WeekEndsOn == nil {
		return 0
	}
	if schools := h.byDays[[2]calendar.Date{*position.WeekStartsOn, *position.WeekEndsOn}]; len(schools) == 1 {
		return schools[0]
	}
	return 0
}

// BookingAccounts serves parentbilling.BookingAccountService.
type BookingAccounts struct {
	store    BookingAccountStore
	settings parentbilling.DatevBillingSettings
	schools  parentbilling.CareSchools
}

var _ parentbilling.BookingAccountService = (*BookingAccounts)(nil)

// NewBookingAccounts builds the accounts page over the DATEV settings and
// the schools of the Träger.
func NewBookingAccounts(store BookingAccountStore, settings parentbilling.DatevBillingSettings, schools parentbilling.CareSchools) *BookingAccounts {
	if store == nil || settings == nil || schools == nil {
		panic("parent billing: the booking accounts need their store, settings and schools")
	}
	return &BookingAccounts{store: store, settings: settings, schools: schools}
}

// BookingAccounts reads the page of the accounts.
func (b *BookingAccounts) BookingAccounts(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.BookingAccounts, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	schools, err := b.schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.BookingAccounts{}, fmt.Errorf("parent billing: read the schools of the Träger: %w", err)
	}
	offerings, err := b.store.BookingOfferings(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	assignments, err := b.store.BookingAssignments(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	datev, err := b.settings.DatevSettings(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.BookingAccounts{}, fmt.Errorf("parent billing: read the DATEV settings: %w", err)
	}
	own := make(map[int64]bool, len(schools))
	for _, school := range schools {
		own[school.ID] = true
	}
	ownOfferings := []parentbilling.BookingOffering{}
	for _, offering := range offerings {
		if own[offering.SchoolID] {
			ownOfferings = append(ownOfferings, offering)
		}
	}
	slices.SortFunc(schools, func(a, c parentbilling.CareSchool) int { return strings.Compare(a.Name, c.Name) })
	page := parentbilling.BookingAccounts{
		StandardAccount: strings.TrimSpace(datev.RevenueAccount.Value), CostCenterRequired: datev.CostCenterRequired,
		Schools: schools, Offerings: ownOfferings, RevenueAccounts: assignments.RevenueAccounts, CostCenters: assignments.CostCenters,
	}
	if page.Schools == nil {
		page.Schools = []parentbilling.CareSchool{}
	}
	if page.RevenueAccounts == nil {
		page.RevenueAccounts = []parentbilling.RevenueAccount{}
	}
	if page.CostCenters == nil {
		page.CostCenters = []parentbilling.CostCenter{}
	}
	return page, nil
}

// ownScope checks that a scope names a Leistung, a school or an Angebot of
// the Träger; a cost center has no Leistung scope but the holiday care, a
// revenue account no school.
func (b *BookingAccounts) ownScope(ctx context.Context, organizationID int64, scope string, scopeID int64, costCenter bool) error {
	switch {
	case scope == parentbilling.BookingScopeHoliday && scopeID == 0,
		!costCenter && scopeID == 0 && (scope == parentbilling.BookingScopeCare || scope == parentbilling.BookingScopeMeals):
		return nil
	case scopeID <= 0:
		return parentbilling.ErrBookingScopeNotFound
	case scope == parentbilling.BookingScopeSchool && costCenter:
		return b.ownSchool(ctx, organizationID, scopeID)
	case scope == parentbilling.BookingScopeOffering:
		return b.ownOffering(ctx, organizationID, scopeID)
	}
	return parentbilling.ErrBookingScopeNotFound
}

func (b *BookingAccounts) ownSchoolIDs(ctx context.Context, organizationID int64) (map[int64]bool, error) {
	schools, err := b.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the schools of the Träger: %w", err)
	}
	own := make(map[int64]bool, len(schools))
	for _, school := range schools {
		own[school.ID] = true
	}
	return own, nil
}

func (b *BookingAccounts) ownSchool(ctx context.Context, organizationID, schoolID int64) error {
	own, err := b.ownSchoolIDs(ctx, organizationID)
	if err != nil {
		return err
	}
	if !own[schoolID] {
		return parentbilling.ErrBookingScopeNotFound
	}
	return nil
}

// ownOffering checks that an Angebot has a price at a school of the Träger.
func (b *BookingAccounts) ownOffering(ctx context.Context, organizationID, offeringID int64) error {
	own, err := b.ownSchoolIDs(ctx, organizationID)
	if err != nil {
		return err
	}
	offerings, err := b.store.BookingOfferings(ctx, organizationID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(offerings, func(offering parentbilling.BookingOffering) bool {
		return offering.CareOfferingID == offeringID && own[offering.SchoolID]
	}) {
		return parentbilling.ErrBookingScopeNotFound
	}
	return nil
}

// SetRevenueAccount sets or removes the revenue account of a scope.
func (b *BookingAccounts) SetRevenueAccount(ctx context.Context, reviewer parentbilling.HolidayReviewer, scope string, scopeID int64, account string,
) (parentbilling.BookingAccounts, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	if err := b.ownScope(ctx, reviewer.OrganizationID, scope, scopeID, false); err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	account = strings.TrimSpace(account)
	switch {
	case account == "":
		err := b.store.DeleteRevenueAccount(ctx, reviewer.OrganizationID, scope, scopeID)
		if err != nil {
			return parentbilling.BookingAccounts{}, err
		}
	case !validRevenueAccount(account):
		return parentbilling.BookingAccounts{}, validation.Errors{"account": errors.New("must be up to 8 digits and not zero")}
	default:
		if err := b.store.SetRevenueAccount(ctx, reviewer.OrganizationID, reviewer.AccountID, scope, scopeID, account); err != nil {
			return parentbilling.BookingAccounts{}, err
		}
	}
	return b.BookingAccounts(ctx, reviewer)
}

// AddCostCenter adds a cost center of a school, the holiday care or an
// Angebot from its day on.
func (b *BookingAccounts) AddCostCenter(ctx context.Context, reviewer parentbilling.HolidayReviewer, center parentbilling.CostCenter,
) (parentbilling.BookingAccounts, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	center.Code = strings.TrimSpace(center.Code)
	errs := validation.Errors{}
	if center.ValidFrom.IsZero() {
		errs["valid_from"] = errors.New("is required")
	}
	if !validCostCenter(center.Code) {
		errs["code"] = errors.New("must be up to 36 letters, digits, _ or spaces")
	}
	if len(errs) > 0 {
		return parentbilling.BookingAccounts{}, errs
	}
	if err := b.ownScope(ctx, reviewer.OrganizationID, center.Scope, center.ScopeID, true); err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	if err := b.store.InsertCostCenter(ctx, reviewer.OrganizationID, reviewer.AccountID, center); err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	return b.BookingAccounts(ctx, reviewer)
}

// DeleteCostCenter removes a cost center of the Träger; positions issued
// with it keep it.
func (b *BookingAccounts) DeleteCostCenter(ctx context.Context, reviewer parentbilling.HolidayReviewer, id int64) (parentbilling.BookingAccounts, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	// The cost center must be one of the Träger's; the store names the
	// Träger again in its statement.
	assignments, err := b.store.BookingAssignments(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	if id <= 0 || !slices.ContainsFunc(assignments.CostCenters, func(c parentbilling.CostCenter) bool { return c.ID == id }) {
		return parentbilling.BookingAccounts{}, parentbilling.ErrCostCenterNotFound
	}
	if err := b.store.DeleteCostCenter(ctx, reviewer.OrganizationID, id); err != nil {
		return parentbilling.BookingAccounts{}, err
	}
	return b.BookingAccounts(ctx, reviewer)
}
