package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// Class letters (#4800): a Lehrkraft writes to the parents of one of her
// classes from moto schule. The letter is an ordinary parent announcement —
// same audience, read receipts, Erklärung, e-mail and push — with a sender
// and exactly one class target. The audience stays the class target, so
// families the school later reaches without an OGS place (#4803) receive the
// letter without a change here.

// normalizeClassLetter checks the shape of a class letter after the common
// normalisation and trims its sender. sender is the Lehrkraft the letter
// comes from; nil leaves an OGS announcement untouched.
func normalizeClassLetter(ctx context.Context, in *Input, sender *ClassLetterSender, targets []*StaffParentAnnouncementTarget) error {
	if sender == nil {
		if _, viaSchool := communication.ClassLetterAuthorFrom(ctx); viaSchool {
			return fmt.Errorf("%w: moto schule writes class letters only", ErrValidation)
		}
		return nil
	}
	name := strings.TrimSpace(sender.Name)
	class := strings.TrimSpace(sender.SchoolClass)
	if name == "" || class == "" {
		return fmt.Errorf("%w: a class letter needs its sender and class", ErrValidation)
	}
	if len(targets) != 1 || targets[0].TargetType != StaffAnnouncementTargetClass ||
		targets[0].TargetRefText == nil || !strings.EqualFold(*targets[0].TargetRefText, class) {
		return fmt.Errorf("%w: a class letter goes to exactly its own class", ErrValidation)
	}
	if in.ResponseType != "" && in.ResponseType != StaffParentAnnouncementResponseNone {
		return fmt.Errorf("%w: a class letter cannot be a poll", ErrValidation)
	}
	// The recipients are the parents with portal access to a child of the
	// class; the wider "all contacts" mail is the OGS's decision.
	if in.EmailAudience != StaffEmailAudiencePortalOnly {
		return fmt.Errorf("%w: a class letter reaches the portal audience only", ErrValidation)
	}
	in.Sender = &ClassLetterSender{Name: name, SchoolClass: class}
	return nil
}

// stampClassLetterSender records the Lehrkraft on a new class letter.
func stampClassLetterSender(a *StaffParentAnnouncement, sender *ClassLetterSender) {
	if sender == nil {
		return
	}
	a.SenderName = &sender.Name
	a.SenderSchoolClass = &sender.SchoolClass
}

// classLetterSenderOf is the sender a stored class letter keeps.
func classLetterSenderOf(a *StaffParentAnnouncement) *ClassLetterSender {
	if a.SenderSchoolClass == nil {
		return nil
	}
	sender := &ClassLetterSender{SchoolClass: *a.SenderSchoolClass}
	if a.SenderName != nil {
		sender.Name = *a.SenderName
	}
	return sender
}

// guardClassLetterWrite keeps the two authors apart (#4800): a class letter
// changes only through moto schule and only for its author, an OGS
// announcement never through moto schule.
func guardClassLetterWrite(ctx context.Context, a *StaffParentAnnouncement) error {
	author, viaSchool := communication.ClassLetterAuthorFrom(ctx)
	switch {
	case a.SenderSchoolClass != nil && (!viaSchool || author != a.CreatedBy):
		return ErrClassLetterReadOnly
	case a.SenderSchoolClass == nil && viaSchool:
		return ErrNotFound
	}
	return nil
}

// classLetterMailIntro names the sender in the mail (#4800): parents read
// who wrote, for which class and at which school, before the usual line.
// The intro follows the salutation, so it starts in lower case.
func classLetterMailIntro(a *StaffParentAnnouncement, schoolName, intro string) string {
	if a.SenderSchoolClass == nil || a.SenderName == nil {
		return intro
	}
	from := fmt.Sprintf("diese Nachricht kommt von %s, Klasse %s", *a.SenderName, *a.SenderSchoolClass)
	if schoolName != "" {
		from += ", " + schoolName
	}
	if intro == "" {
		intro = "Den vollständigen Inhalt finden Sie im Eltern-Portal."
	}
	runes := []rune(intro)
	return from + ". " + strings.ToUpper(string(runes[0])) + string(runes[1:])
}
