package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// EnrollmentSettingsReads is the slice of the settings service the
// Enrollment bindings resolve through.
type EnrollmentSettingsReads interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveString(ctx context.Context, key string) (string, error)
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	LockClassCollectionPair(ctx context.Context) error
}

// EnrollmentCollectionSettings resolves the grade and class collection
// settings the phase and schema guards check.
type EnrollmentCollectionSettings struct{ Settings EnrollmentSettingsReads }

func (s EnrollmentCollectionSettings) CollectGradeLevel(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectGradeLevel)
}

func (s EnrollmentCollectionSettings) CollectSchoolClass(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectSchoolClass)
}

func (s EnrollmentCollectionSettings) GradeLevelMax(ctx context.Context) (int, error) {
	return s.Settings.ResolveInt(ctx, configModels.KeyEnrollmentGradeLevelMax)
}

func (s EnrollmentCollectionSettings) LockClassCollectionPair(ctx context.Context) error {
	return s.Settings.LockClassCollectionPair(ctx)
}

// enrollmentNotificationSettings resolves the decision notification mode.
type enrollmentNotificationSettings struct {
	settings interface {
		ResolveString(ctx context.Context, key string) (string, error)
	}
}

// NewEnrollmentNotifications composes Enrollment's mail branding and decision
// notifications over the Delivery outbox.
func NewEnrollmentNotifications(modes enrollmentCompose.NotificationModePin, settings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}, outbox MailOutbox, schools enrollmentOwner.SchoolDirectory) enrollmentOwner.Notifications {
	return enrollmentCompose.NewNotifications(enrollmentCompose.NotificationDependencies{
		Modes:       modes,
		Settings:    enrollmentNotificationSettings{settings: settings},
		Outbox:      enrollmentMailOutbox{outbox: outbox},
		Schools:     schools,
		Fingerprint: securityruntime.Fingerprint,
		Branding:    EnrollmentMailBranding,
	})
}

func (s enrollmentNotificationSettings) NotifyPerDecision(ctx context.Context) (string, error) {
	return s.settings.ResolveString(ctx, configModels.KeyEnrollmentNotifyPerDecision)
}

func (s enrollmentNotificationSettings) CareFacilityTerm(ctx context.Context) (string, error) {
	return s.settings.ResolveString(ctx, configModels.KeyEnrollmentCareFacilityTerm)
}

// EnrollmentCaptchaSettings resolves whether the school in context requires
// the captcha. The Turnstile keys are platform configuration: one widget
// serves every school.
type EnrollmentCaptchaSettings struct {
	Settings  EnrollmentSettingsReads
	SiteKey   string
	SecretKey string
}

func (s EnrollmentCaptchaSettings) CaptchaRequired(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyEnrollmentRequireCaptcha)
}

func (s EnrollmentCaptchaSettings) CaptchaSecretKey(context.Context) (string, error) {
	return s.SecretKey, nil
}

func (s EnrollmentCaptchaSettings) CaptchaSiteKey(context.Context) (string, error) {
	return s.SiteKey, nil
}

// enrollmentPhaseOfferings reads the Care Plan offerings of a phase.
type enrollmentPhaseOfferings struct {
	carePlan interface {
		ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
		CountCareOfferingsByPhase(context.Context, int64) (int, error)
		CareOfferingBookingsForPhase(context.Context, int64) ([]careplan.CareOfferingBooking, error)
	}
}

func (o enrollmentPhaseOfferings) CareOfferingBookingsForPhase(ctx context.Context, phaseID int64) ([]enrollmentOwner.OfferingBooking, error) {
	rows, err := o.carePlan.CareOfferingBookingsForPhase(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to list care offering bookings by phase: %w", err)
	}
	return enrollmentBookings(rows), nil
}

func (o enrollmentPhaseOfferings) OfferingIDsForPhase(ctx context.Context, phaseID int64) ([]int64, error) {
	offerings, err := o.carePlan.ListCareOfferings(ctx, careplan.CareOfferingFilter{PhaseIDs: []int64{phaseID}, Order: careplan.OfferingOrderCatalog})
	if err != nil {
		return nil, fmt.Errorf("failed to list care offerings by phase: %w", err)
	}
	ids := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		ids = append(ids, offering.ID)
	}
	return ids, nil
}

func (o enrollmentPhaseOfferings) CountOfferingsForPhase(ctx context.Context, phaseID int64) (int, error) {
	count, err := o.carePlan.CountCareOfferingsByPhase(ctx, phaseID)
	if err != nil {
		return 0, fmt.Errorf("failed to count care offerings by phase: %w", err)
	}
	return count, nil
}

// parentCarePeriods answers the parent portal's care periods from
// Enrollment.
type parentCarePeriods struct {
	owner interface {
		StudentCarePeriods(context.Context, int64) ([]*enrollmentOwner.StudentCarePeriod, error)
	}
}

func (p parentCarePeriods) CarePeriods(ctx context.Context, studentID int64) ([]*parentportal.CarePeriod, error) {
	periods, err := enrollmentOwner.StudentCarePeriodRecords(ctx, p.owner, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]*parentportal.CarePeriod, 0, len(periods))
	for _, period := range periods {
		result = append(result, &parentportal.CarePeriod{
			RequestChildID: period.RequestChildID, RequestID: period.RequestID, PhaseID: period.PhaseID,
			PhaseName: period.PhaseName, ServiceStartDate: period.ServiceStartDate, ServiceEndDate: period.ServiceEndDate,
		})
	}
	return result, nil
}

// parentOfferingHistory answers the parent portal's booked offerings from
// Enrollment.
type parentOfferingHistory struct {
	owner interface {
		RequestChildOfferingHistory(context.Context, int64) ([]*enrollmentOwner.RequestChildOffering, error)
	}
}

func (p parentOfferingHistory) OfferingHistory(ctx context.Context, requestChildID int64) ([]*parentportal.OfferingBooking, error) {
	links, err := enrollmentOwner.OfferingHistoryRecords(ctx, p.owner, requestChildID)
	if err != nil {
		return nil, err
	}
	result := make([]*parentportal.OfferingBooking, 0, len(links))
	for _, link := range links {
		if link == nil {
			result = append(result, nil)
			continue
		}
		result = append(result, &parentportal.OfferingBooking{
			CareOfferingID: link.CareOfferingID, SelectedDays: link.SelectedDays,
			ValidFrom: link.ValidFrom, ValidUntil: link.ValidUntil,
		})
	}
	return result, nil
}

// enrollmentMailOutbox puts Enrollment's mails on Delivery's outbox.
type enrollmentMailOutbox struct {
	outbox MailOutbox
}

func (o enrollmentMailOutbox) EnqueueMail(ctx context.Context, mail enrollmentCompose.Mail) error {
	return o.outbox.EnqueueOutbox(ctx, deliveryCompose.MailEnqueueRequest{
		Kind: mail.Kind, Payload: mail.Payload, RelatedEntityType: mail.RelatedEntityType,
		RelatedEntityID: mail.RelatedEntityID, IdempotencyKey: mail.IdempotencyKey,
	})
}

// HolidayAcceptanceAccounts tells the acceptance mail of a holiday
// registration whether its address has a parent account (#6672): an account
// of the address that holds the guardian role at a live school, the rule the
// parents portal signs in by. Identity & Access serves it in the worker's
// administrative scope.
type HolidayAcceptanceAccounts interface {
	FindAccountByEmail(ctx context.Context, email string) (identityaccess.Account, error)
	FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error)
}

// holidayAcceptanceAccounts maps Identity account lookups to Enrollment's
// account-eligibility port in the worker's administrative scope. links
// issues the setup link of a parent account for an address without one (ADR
// 0216 point 11); nil offers none.
type holidayAcceptanceAccounts struct {
	accounts HolidayAcceptanceAccounts
	links    enrollmentOwner.HolidayParentAccountLinks
}

// newHolidayAcceptanceAccounts binds the acceptance mail's links: the
// account lookups of Identity & Access and, where it serves them, the setup
// links of a parent account at a Träger with the parents portal's address.
func newHolidayAcceptanceAccounts(accounts HolidayAcceptanceAccounts, parentsURL string) holidayAcceptanceAccounts {
	bound := holidayAcceptanceAccounts{accounts: accounts}
	if setups, ok := accounts.(identityaccess.HolidayParentSetups); ok && strings.TrimSpace(parentsURL) != "" {
		bound.links = NewHolidayParentAccountLinks(setups, parentsURL)
	}
	return bound
}

// HasParentAccount: an account of the address that the parents portal signs
// in, by the guardian role at a live school or by its access at an active
// Träger (ADR 0216).
func (a holidayAcceptanceAccounts) HasParentAccount(ctx context.Context, address string) (bool, error) {
	if a.accounts == nil {
		return false, nil
	}
	account, err := a.accounts.FindAccountByEmail(ctx, address)
	if errors.Is(err, identityaccess.ErrAccountNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	guardian, _, err := a.accounts.FindGuardianTenant(ctx, account.ID)
	if err != nil || guardian {
		return guardian, err
	}
	setups, ok := a.accounts.(identityaccess.HolidayParentSetups)
	if !ok {
		return false, nil
	}
	return setups.HasTraegerParentAccess(ctx, account.ID)
}

// HolidayParentSetupURL is the setup link of a parent account at the Träger;
// without the setup links bound the mail offers none.
func (a holidayAcceptanceAccounts) HolidayParentSetupURL(address string, organizationID int64) (string, error) {
	if a.links == nil {
		return "", nil
	}
	link, err := a.links.HolidayParentAccountLink(address, organizationID)
	return link.SetupURL, err
}

// holidayParentAccountLinks binds the offer of a parent account (ADR 0216
// point 4) to Identity & Access's setup link, which only signs and so runs
// in the registration's Träger transaction, and to the parents portal.
type holidayParentAccountLinks struct {
	setups     identityaccess.HolidayParentSetups
	parentsURL string
}

// NewHolidayParentAccountLinks binds the setup link of the offer mail: the
// page /ferienkonto/{token} and the sign-in /login of the parents portal.
func NewHolidayParentAccountLinks(setups identityaccess.HolidayParentSetups, parentsURL string) enrollmentOwner.HolidayParentAccountLinks {
	return holidayParentAccountLinks{setups: setups, parentsURL: strings.TrimRight(parentsURL, "/")}
}

func (l holidayParentAccountLinks) HolidayParentAccountLink(email string, organizationID int64) (enrollmentOwner.HolidayParentAccountLink, error) {
	if l.setups == nil || l.parentsURL == "" {
		return enrollmentOwner.HolidayParentAccountLink{}, errors.New("holiday registration: the setup link of a parent account is not configured")
	}
	token, err := l.setups.IssueHolidayParentSetupToken(email, organizationID)
	if err != nil {
		return enrollmentOwner.HolidayParentAccountLink{}, err
	}
	return enrollmentOwner.HolidayParentAccountLink{
		SetupURL: l.parentsURL + "/ferienkonto/" + url.PathEscape(token),
		LoginURL: l.parentsURL + "/login",
	}, nil
}

// enrollmentMailRenderers registers Enrollment's renderers by outbox kind.
// Delivery chooses the sender from the mail's school name.
func enrollmentMailRenderers(renderers enrollmentOwner.MailRenderers, defaultFrom email.Email) map[string]deliveryCompose.MailRenderer {
	render := func(renderer enrollmentOwner.MailRenderer) deliveryCompose.MailRenderer {
		return deliveryCompose.MailRendererFunc(func(ctx context.Context, intent *deliveryCompose.MailIntent) (*email.Message, error) {
			payload, err := json.Marshal(intent.Payload)
			if err != nil {
				return nil, fmt.Errorf("%s payload: %w", intent.Kind, err)
			}
			mail, err := renderer(ctx, intent.Kind, payload)
			if err != nil {
				return nil, err
			}
			return &email.Message{
				From: email.NewEmail(deliveryModule.SchoolSender(mail.SenderName, defaultFrom.Name, defaultFrom.Address)), To: email.NewEmail("", mail.Recipient),
				Subject: mail.Subject, Template: mail.Template, Content: mail.Content,
			}, nil
		})
	}
	return map[string]deliveryCompose.MailRenderer{
		enrollmentOwner.MailKindSubmitted:                render(renderers.Submitted),
		enrollmentOwner.MailKindAdminNotification:        render(renderers.AdminNotification),
		enrollmentOwner.MailKindApproved:                 render(renderers.Approved),
		enrollmentOwner.MailKindWaitlisted:               render(renderers.Waitlisted),
		enrollmentOwner.MailKindRejected:                 render(renderers.Rejected),
		enrollmentOwner.MailKindDecisionDigest:           render(renderers.DecisionDigest),
		enrollmentOwner.MailKindChangeRequestSubmitted:   render(renderers.ChangeRequestSubmitted),
		enrollmentOwner.MailKindChangeRequestQuestion:    render(renderers.ChangeRequestQuestion),
		enrollmentOwner.MailKindChangeRequestParentReply: render(renderers.ChangeRequestParentReply),
		enrollmentOwner.MailKindChangeRequestApproved:    render(renderers.ChangeRequestApproved),
		enrollmentOwner.MailKindChangeRequestRejected:    render(renderers.ChangeRequestRejected),
		enrollmentOwner.MailKindRolloverOptIn:            render(renderers.RolloverOptIn),
		enrollmentOwner.MailKindRolloverOptOut:           render(renderers.RolloverOptOut),
		// The receipt of a public holiday registration is a mail of the
		// Träger, sent with the Träger's name (#4661).
		enrollmentOwner.MailKindHolidayRegistrationReceived: render(renderers.HolidayRegistrationReceived),
		// A child that moved up from the waiting list of a holiday week
		// (#4699): a mail of the Träger like the receipt.
		enrollmentOwner.MailKindHolidayPlaceGranted: render(renderers.HolidayPlaceGranted),
		// A closing day in a booked holiday week (#6434): a mail of the
		// Träger like the receipt.
		enrollmentOwner.MailKindHolidayClosureNotice: render(renderers.HolidayClosureNotice),
		// The acceptance of a holiday registration (#6672): a mail of the
		// Träger like the receipt.
		enrollmentOwner.MailKindHolidayRegistrationAccepted: render(renderers.HolidayRegistrationAccepted),
		// The offer of a parent account a holiday registration mails (ADR
		// 0216): a mail of the Träger like the receipt.
		enrollmentOwner.MailKindHolidayParentAccountOffer: render(renderers.HolidayParentAccountOffer),
	}
}

// EnrollmentMailBranding binds Enrollment's logo port to Delivery.
func EnrollmentMailBranding(baseURL, schoolImageURL string) (string, string) {
	return deliveryCompose.SchoolLogoURL(baseURL, schoolImageURL), deliveryCompose.MotoLogoURL(baseURL)
}

// enrollmentPhaseStudents binds the phase edit's pending-window move to
// School Membership's guarded command (#6505, ADR 0202). A window the command
// refuses as inverted is a conflict of the phase edit.
type enrollmentPhaseStudents struct {
	membership schoolmembership.PendingWindows
}

// NewEnrollmentPhaseStudents binds the phase edit to School Membership.
func NewEnrollmentPhaseStudents(membership schoolmembership.PendingWindows) enrollmentOwner.PhaseStudents {
	return enrollmentPhaseStudents{membership: membership}
}

func (s enrollmentPhaseStudents) MovePendingWindow(ctx context.Context, move enrollmentOwner.PendingWindowMove) (bool, error) {
	moved, err := s.membership.MovePendingWindow(ctx, schoolmembership.PendingWindowMove{
		StudentID:  move.StudentID,
		FromBefore: calendar.Date(move.FromBefore), FromAfter: calendar.Date(move.FromAfter),
		UntilBefore: calendar.Date(move.UntilBefore), UntilAfter: calendar.Date(move.UntilAfter),
		OtherStarts: calendarDates(move.OtherStarts), OtherEnds: calendarDates(move.OtherEnds),
	})
	var conflict *schoolmembership.PendingWindowConflictError
	if errors.As(err, &conflict) {
		return false, fmt.Errorf("%w: student %d", enrollmentOwner.ErrPhasePendingChildWindowConflict, conflict.StudentID)
	}
	return moved, err
}

func calendarDates(days []enrollmentOwner.Date) []calendar.Date {
	out := make([]calendar.Date, 0, len(days))
	for _, day := range days {
		out = append(out, calendar.Date(day))
	}
	return out
}

// enrollmentPhaseBookings binds the phase edit's booking bounds to Care
// Plan's guarded command (#6597, ADR 0205). A booking the command refuses as
// empty is a conflict of the phase edit.
type enrollmentPhaseBookings struct {
	bookings careplan.PhaseBookingBounds
}

// NewEnrollmentPhaseBookings binds the phase edit to Care Plan's bookings.
func NewEnrollmentPhaseBookings(bookings careplan.PhaseBookingBounds) enrollmentOwner.PhaseBookings {
	return enrollmentPhaseBookings{bookings: bookings}
}

func (b enrollmentPhaseBookings) FollowPhaseBookingBounds(ctx context.Context, move enrollmentOwner.PhaseBookingMove) error {
	err := b.bookings.FollowPhaseBookingBounds(ctx, careplan.PhaseBoundsMove(move))
	var conflict *careplan.PhaseBookingBoundsConflictError
	if errors.As(err, &conflict) {
		return &enrollmentOwner.PhaseBookingBoundsConflictError{RequestChildID: conflict.RequestChildID}
	}
	return err
}
