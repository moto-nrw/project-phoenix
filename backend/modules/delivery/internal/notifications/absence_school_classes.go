package notifications

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
)

// Copy for the school-portal notice (#4792).
//
// Unlike the OGS notice it names the class: a Lehrkraft or the
// Schulverwaltung needs to know which class to open, and a class is no
// personal data. It still names no child and no reason, because the push
// lands on a lock screen.
const (
	schoolClassAbsenceTitleSick    = "Krankmeldung"
	schoolClassAbsenceTitleExcused = "Entschuldigung"
	schoolClassAbsenceBodySick     = "Neue Krankmeldung in %s"
	schoolClassAbsenceBodyExcused  = "Neue Entschuldigung in %s"
)

// permissionClassDayAllClasses restates the permission registry's name, which
// this package may not import: the Schulverwaltung (#4789) reaches every class
// with it. TestSchoolClassAbsencePermissionName pins it to the registry.
const permissionClassDayAllClasses = "class_day:all_classes"

// StudentClassTeachers is one reported child's class as entered and the
// accounts of the Lehrkräfte assigned to it (education.class_teachers).
type StudentClassTeachers struct {
	SchoolClass string
	AccountIDs  []int64
}

// SchoolClassReach names the class of each reported child and its assigned
// Lehrkräfte. The root binds it over People Directory, School Membership and
// the staff accounts. A child without a class is missing from the result;
// SchoolClass is trimmed and never empty.
type SchoolClassReach interface {
	StudentClassTeachers(ctx context.Context, studentIDs []int64) (map[int64]StudentClassTeachers, error)
}

// schoolClassConsent is the consent enforcement point every producer uses: it
// applies the tenant gate and the person's decision.
type schoolClassConsent interface {
	FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
}

// schoolWideAccounts is the slice of Identity & Access that tells which
// accounts of the school hold a permission, as for the opt-in e-mail.
type schoolWideAccounts interface {
	ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error)
	FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error)
}

// SchoolClassRecipients are the collaborators of the moto schule notice.
type SchoolClassRecipients struct {
	Reach   SchoolClassReach
	Consent schoolClassConsent
	// Subscribers lists the accounts that switched the type on. It is the
	// cheap first question (nobody, no lookup at all) and the source of the
	// Schulverwaltung, narrowed by Accounts and Permissions.
	Subscribers EmailSubscribers
	Accounts    schoolWideAccounts
	Permissions PermissionMatch
}

// WithSchoolClassRecipients adds the school-portal notice to the absence
// notifier. Without it the notifier only informs the OGS.
func WithSchoolClassRecipients(recipients SchoolClassRecipients) AbsenceNotifierOption {
	return func(n *absenceNotifier) {
		n.school = &recipients
	}
}

// schoolClassEvents builds one event per reported child for the accounts that
// reach its class and switched the type on: its Lehrkräfte and the
// Schulverwaltung. One child is one notice, never a bundle.
func (n *absenceNotifier) schoolClassEvents(ctx context.Context, report AbsenceReport) ([]Event, error) {
	school := n.school
	if school == nil {
		return nil, nil
	}
	subscribers, err := school.Subscribers.ListOptedIn(ctx, TypeSchoolClassAbsenceReported)
	if err != nil {
		return nil, fmt.Errorf("list school class subscribers: %w", err)
	}
	if len(subscribers) == 0 {
		return nil, nil
	}
	studentIDs := uniqueInt64s(report.StudentIDs)
	classes, err := school.Reach.StudentClassTeachers(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve school classes: %w", err)
	}
	if len(classes) == 0 {
		return nil, nil
	}
	schoolWide, err := school.schoolWide(ctx, report.TenantID, subscribers)
	if err != nil {
		return nil, err
	}
	agreed, err := school.agreed(ctx, report, classes, schoolWide)
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		class, ok := classes[studentID]
		if !ok {
			continue
		}
		recipients := make([]int64, 0, len(class.AccountIDs)+len(schoolWide))
		for _, accountID := range slices.Concat(class.AccountIDs, schoolWide) {
			if agreed[accountID] {
				recipients = append(recipients, accountID)
			}
		}
		slices.Sort(recipients)
		recipients = slices.Compact(recipients)
		if len(recipients) > 0 {
			events = append(events, schoolClassAbsenceEvent(report, studentID, class.SchoolClass, recipients))
		}
	}
	return events, nil
}

// schoolWide narrows the subscribers to the active accounts holding
// class_day:all_classes, the Schulverwaltung.
func (s *SchoolClassRecipients) schoolWide(ctx context.Context, tenantID int64, subscribers []int64) ([]int64, error) {
	active, err := s.Accounts.ListActiveAccountIDsForTenant(ctx, tenantID, subscribers)
	if err != nil {
		return nil, fmt.Errorf("check school class subscriber memberships: %w", err)
	}
	if len(active) == 0 {
		return nil, nil
	}
	names, err := s.Accounts.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, active, tenantID)
	if err != nil {
		return nil, fmt.Errorf("check school class subscriber permissions: %w", err)
	}
	schoolWide := make([]int64, 0, len(active))
	for _, accountID := range active {
		if s.Permissions.permits(permissionClassDayAllClasses, names[accountID]) {
			schoolWide = append(schoolWide, accountID)
		}
	}
	return schoolWide, nil
}

// agreed applies the single consent enforcement point to everyone reaching a
// reported class, minus the people who originated the report.
func (s *SchoolClassRecipients) agreed(ctx context.Context, report AbsenceReport, classes map[int64]StudentClassTeachers, schoolWide []int64) (map[int64]bool, error) {
	candidates := make(map[int64]struct{}, len(schoolWide))
	for _, accountID := range schoolWide {
		candidates[accountID] = struct{}{}
	}
	for _, class := range classes {
		for _, accountID := range class.AccountIDs {
			candidates[accountID] = struct{}{}
		}
	}
	delete(candidates, report.ActorAccountID)
	for _, accountID := range report.ExcludedAccountIDs {
		delete(candidates, accountID)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	optedIn, err := s.Consent.FilterOptedIn(ctx, TypeSchoolClassAbsenceReported, sortedInt64Set(candidates))
	if err != nil {
		return nil, fmt.Errorf("filter school class consent: %w", err)
	}
	agreed := make(map[int64]bool, len(optedIn))
	for _, accountID := range optedIn {
		agreed[accountID] = true
	}
	return agreed, nil
}

func schoolClassAbsenceEvent(report AbsenceReport, studentID int64, schoolClass string, recipients []int64) Event {
	title, body := schoolClassAbsenceTitleSick, schoolClassAbsenceBodySick
	if report.Status == AbsenceStatusExcused {
		title, body = schoolClassAbsenceTitleExcused, schoolClassAbsenceBodyExcused
	}
	scoped := report
	scoped.StudentIDs = []int64{studentID}
	return Event{
		Type:           TypeSchoolClassAbsenceReported,
		IdempotencyKey: "school-class-" + absenceEventKey(scoped, recipients),
		RelatedType:    "student_absence",
		RelatedID:      studentID,
		Priority:       PriorityNormal,
		Title:          title,
		Body:           fmt.Sprintf(body, schoolClassLabel(schoolClass)),
		SchoolDeepLink: schoolClassDeepLink(schoolClass),
		Audience: Audience{
			TenantID:        report.TenantID,
			Scope:           ScopeStaff,
			StaffAccountIDs: recipients,
		},
	}
}

// schoolClassLabel names a class the way moto schule writes it
// (schoolClassLabel in the frontend): class names are free text, some
// schools store "1a", others "Klasse 1a", and neither may read
// "Klasse Klasse 1a".
func schoolClassLabel(schoolClass string) string {
	if classPrefix.MatchString(schoolClass) {
		return schoolClass
	}
	return "Klasse " + schoolClass
}

var classPrefix = regexp.MustCompile(`(?i)^klasse\b`)

// schoolClassDeepLink opens the class view of moto schule. Class names are
// free text ("5 b/c"), so the name travels encoded in the query.
func schoolClassDeepLink(schoolClass string) string {
	return "/school/klasse?klasse=" + url.QueryEscape(schoolClass)
}
