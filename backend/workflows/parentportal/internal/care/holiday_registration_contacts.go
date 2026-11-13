package care

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
)

// Contacts of the holiday registration prefill (#6682): the other guardians
// of a child the parent may register, as the school knows them. The form
// offers a full guardian as the second custodian and every emergency or
// pickup contact as a contact, each with the email and phone the school
// knows. The offered children come from the account's child list, which
// requires parent_portal.access. Unlike the guardian list, the prefill does
// not redact a guardian with an own account or a contact shared with
// another family: the parent registers the child with exactly these
// contacts, and the school knows them (product decision on #6682). Social
// workers stay out entirely: the school manages them, they are no family
// contact. Health data is not part of the prefill (Art. 9 DSGVO, issue
// decision option A).

// HolidayRegistrationAddress is the postal address of the calling guardian.
type HolidayRegistrationAddress struct {
	Street     string
	PostalCode string
	City       string
}

// HolidayRegistrationContact is another guardian of an offered child.
// Email and Phone are empty when the school knows none. FullGuardian
// marks a custodian (primary, legal or co-guardian), Emergency and Pickup the
// child's arrangement at the school.
type HolidayRegistrationContact struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
	Email             string
	Phone             string
	RelationshipType  string
	FullGuardian      bool
	Emergency         bool
	Pickup            bool
}

// attachHolidayRegistrationContacts loads the other guardians of the
// offered children of one school in a fixed number of statements and adds
// them to each child. It runs in the school's tenant transaction, where the
// child's read is already recorded in the data access log.
func (s *Service) attachHolidayRegistrationContacts(ctx context.Context, accountID int64, offered []*HolidayRegistrationChild) error {
	if len(offered) == 0 {
		return nil
	}
	studentIDs := make([]int64, 0, len(offered))
	for _, child := range offered {
		studentIDs = append(studentIDs, child.StudentID)
	}
	links, err := s.StudentGuardianRepo.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return err
	}
	links = sortHolidayContactLinks(links)
	profileIDs := uniqueGuardianProfileIDs(links)
	if len(profileIDs) == 0 {
		return nil
	}
	sources, err := s.loadHolidayContactSources(ctx, profileIDs)
	if err != nil {
		return err
	}
	byStudent := make(map[int64]*HolidayRegistrationChild, len(offered))
	for _, child := range offered {
		byStudent[child.StudentID] = child
	}
	for _, link := range links {
		child := byStudent[link.StudentID]
		if child == nil {
			continue
		}
		profile, ok := sources.profiles[link.GuardianProfileID]
		if !ok {
			// A data-integrity fault, as in the child's guardian list.
			s.Logger.Error("parent holiday registration contact points to missing profile",
				slog.Int64("student_id", link.StudentID),
				slog.Int64("student_guardian_id", link.ID),
				slog.Int64("guardian_profile_id", link.GuardianProfileID),
			)
			return fmt.Errorf("parent: student_guardian %d references missing guardian profile %d", link.ID, link.GuardianProfileID)
		}
		if contact, ok := s.holidayRegistrationContact(profile, link, sources.phones[profile.ID], accountID); ok {
			child.Contacts = append(child.Contacts, contact)
		}
	}
	return nil
}

// holidayContactSources are the batch reads behind the contacts of one school.
type holidayContactSources struct {
	profiles map[int64]*GuardianProfile
	phones   map[int64][]*GuardianPhoneNumber
}

// loadHolidayContactSources reads the profiles and phones of the guardians in
// two statements.
func (s *Service) loadHolidayContactSources(ctx context.Context, profileIDs []int64) (*holidayContactSources, error) {
	profiles, err := s.GuardianProfileRepo.FindByIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	phones, err := s.GuardianPhoneRepo.FindByGuardianIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	return &holidayContactSources{profiles: profiles, phones: phones}, nil
}

// holidayRegistrationContact projects one guardian of a child for the
// prefill. It reports false for the caller, a social worker, and a guardian
// who is neither custodian nor emergency nor pickup contact.
func (s *Service) holidayRegistrationContact(profile *GuardianProfile, link *GuardianRelationship, phones []*GuardianPhoneNumber, accountID int64) (HolidayRegistrationContact, bool) {
	isSelf := profile.AccountID != nil && *profile.AccountID == accountID
	if isSelf || link.GuardianRole == GuardianRoleSocialWorker {
		return HolidayRegistrationContact{}, false
	}
	contact := HolidayRegistrationContact{
		GuardianProfileID: profile.ID, FirstName: profile.FirstName, LastName: profile.LastName,
		RelationshipType: link.RelationshipType, FullGuardian: s.GuardianPolicy.IsFullRole(link.GuardianRole),
		Emergency: link.IsEmergencyContact, Pickup: link.CanPickup,
	}
	if !contact.FullGuardian && !contact.Emergency && !contact.Pickup {
		return HolidayRegistrationContact{}, false
	}
	contact.Email = deref(profile.Email)
	if primary := pickPrimaryPhone(phones, func(phone *GuardianPhoneNumber) bool { return phone.IsPrimary }); primary != nil {
		contact.Phone = primary.PhoneNumber
	}
	return contact, true
}

// sortHolidayContactLinks puts the emergency contacts first, in the school's
// emergency priority, and the other guardians after them by record. The form
// keeps only the first contacts within its limits, so the order decides who
// they are; the read itself has no order.
func sortHolidayContactLinks(links []*GuardianRelationship) []*GuardianRelationship {
	links = slices.DeleteFunc(links, func(link *GuardianRelationship) bool { return link == nil })
	slices.SortStableFunc(links, func(a, b *GuardianRelationship) int {
		switch {
		case a.IsEmergencyContact != b.IsEmergencyContact:
			if a.IsEmergencyContact {
				return -1
			}
			return 1
		case a.IsEmergencyContact && a.EmergencyPriority != b.EmergencyPriority:
			return compareInt64(int64(a.EmergencyPriority), int64(b.EmergencyPriority))
		default:
			return compareInt64(a.ID, b.ID)
		}
	})
	return links
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func uniqueGuardianProfileIDs(links []*GuardianRelationship) []int64 {
	seen := make(map[int64]bool, len(links))
	ids := make([]int64, 0, len(links))
	for _, link := range links {
		if link == nil || seen[link.GuardianProfileID] {
			continue
		}
		seen[link.GuardianProfileID] = true
		ids = append(ids, link.GuardianProfileID)
	}
	return ids
}

// holidayRegistrationAddress is the calling guardian's own postal address
// as the school knows it, also an incomplete one: the form then asks for the
// missing part. Nil when the school knows none.
func holidayRegistrationAddress(profile *GuardianProfile) *HolidayRegistrationAddress {
	address := HolidayRegistrationAddress{
		Street: deref(profile.AddressStreet), PostalCode: deref(profile.AddressPostalCode), City: deref(profile.AddressCity),
	}
	if address == (HolidayRegistrationAddress{}) {
		return nil
	}
	return &address
}
