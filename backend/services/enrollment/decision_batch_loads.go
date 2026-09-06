package enrollment

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/users"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func schemaIDsFromRequests(requests []*enrollmentModels.Request) []int64 {
	ids := make([]int64, 0, len(requests))
	seen := make(map[int64]bool)
	for _, request := range requests {
		if request != nil && request.SchemaID != nil && !seen[*request.SchemaID] {
			seen[*request.SchemaID] = true
			ids = append(ids, *request.SchemaID)
		}
	}
	return ids
}

func findGuardianProfilesByEmails(
	ctx context.Context,
	repo users.GuardianProfileRepository,
	emails []string,
) (map[string]*users.GuardianProfile, error) {
	normalized := make([]string, 0, len(emails))
	seen := make(map[string]bool, len(emails))
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email != "" && !seen[email] {
			seen[email] = true
			normalized = append(normalized, email)
		}
	}
	result := make(map[string]*users.GuardianProfile, len(normalized))
	if len(normalized) == 0 {
		return result, nil
	}
	profiles, err := repo.ListWithOptions(ctx, &modelBase.QueryOptions{
		Filter: modelBase.NewFilter().TrimIn("email", normalized...),
	})
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile != nil && profile.Email != nil {
			result[strings.ToLower(strings.TrimSpace(*profile.Email))] = profile
		}
	}
	return result, nil
}

// SchemaReader supplies immutable versions for decisions and reports.
type SchemaReader interface {
	Schema(context.Context, int64) (*capability.FormSchema, error)
	Schemas(context.Context, []int64) ([]*capability.FormSchema, error)
}

func loadFormSchemasByRequests(
	ctx context.Context,
	repo SchemaReader,
	requests []*enrollmentModels.Request,
) (map[int64]*capability.FormSchema, error) {
	ids := schemaIDsFromRequests(requests)
	result := make(map[int64]*capability.FormSchema, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	if repo == nil {
		return nil, fmt.Errorf("form schema repo not configured")
	}
	rows, err := repo.Schemas(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, schema := range rows {
		result[schema.ID] = cloneSchema(schema)
	}
	for _, id := range ids {
		if result[id] == nil {
			return nil, fmt.Errorf("form schema %d not found", id)
		}
	}
	return result, nil
}

func int64SetKeys(values map[int64]struct{}) []int64 {
	ids := make([]int64, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	return ids
}

func careOfferingMap(rows []*enrollmentModels.CareOffering) map[int64]*enrollmentModels.CareOffering {
	result := make(map[int64]*enrollmentModels.CareOffering, len(rows))
	for _, row := range rows {
		if row != nil {
			result[row.ID] = row
		}
	}
	return result
}

func phaseMap(rows []*capability.Phase) map[int64]*capability.Phase {
	result := make(map[int64]*capability.Phase, len(rows))
	for _, row := range rows {
		if row != nil {
			result[row.ID] = row
		}
	}
	return result
}

type phoneOnlyContactProfiles struct {
	profiles        map[int64]*users.GuardianProfile
	phonesByProfile map[int64]map[string]bool
}

func (s *decisionService) loadPhoneOnlyContactProfiles(
	ctx context.Context,
	studentID int64,
) (*phoneOnlyContactProfiles, error) {
	result := &phoneOnlyContactProfiles{profiles: map[int64]*users.GuardianProfile{}, phonesByProfile: map[int64]map[string]bool{}}
	if studentID <= 0 || s.StudentGuardianRepo == nil || s.GuardianProfileRepo == nil || s.GuardianPhoneRepo == nil {
		return result, nil
	}
	links, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	profileIDs := contactOnlyProfileIDs(links)
	result.profiles, err = s.GuardianProfileRepo.FindByIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	phones, err := s.GuardianPhoneRepo.FindByGuardianIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	for profileID, rows := range phones {
		result.phonesByProfile[profileID] = guardianPhoneSet(rows)
	}
	return result, nil
}

func contactOnlyProfileIDs(links []*users.StudentGuardian) []int64 {
	ids := make([]int64, 0, len(links))
	seen := map[int64]bool{}
	for _, link := range links {
		if link == nil || link.IsPrimary || authorize.IsFullGuardianRole(link.GuardianRole) || link.GuardianProfileID <= 0 || seen[link.GuardianProfileID] {
			continue
		}
		seen[link.GuardianProfileID] = true
		ids = append(ids, link.GuardianProfileID)
	}
	return ids
}

func guardianPhoneSet(rows []*users.GuardianPhoneNumber) map[string]bool {
	phones := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row != nil {
			phones[strings.TrimSpace(row.PhoneNumber)] = true
		}
	}
	return phones
}

func (profiles *phoneOnlyContactProfiles) match(entry capability.ContactEntry) []*users.GuardianProfile {
	firstName := contactIdentityName(entry.FirstName)
	lastName := contactIdentityName(entry.LastName)
	phones := contactIdentityPhones(entry)
	if firstName == "" || lastName == "" || len(phones) == 0 {
		return nil
	}
	matches := make([]*users.GuardianProfile, 0, 1)
	for profileID, profile := range profiles.profiles {
		if profile == nil || contactIdentityName(profile.FirstName) != firstName || contactIdentityName(profile.LastName) != lastName {
			continue
		}
		for phone := range profiles.phonesByProfile[profileID] {
			if phones[phone] {
				matches = append(matches, profile)
				break
			}
		}
	}
	return matches
}

func (profiles *phoneOnlyContactProfiles) add(profile *users.GuardianProfile, entry capability.ContactEntry) {
	if profile == nil || profile.ID <= 0 {
		return
	}
	profiles.profiles[profile.ID] = profile
	phones := profiles.phonesByProfile[profile.ID]
	if phones == nil {
		phones = map[string]bool{}
		profiles.phonesByProfile[profile.ID] = phones
	}
	for phone := range contactIdentityPhones(entry) {
		phones[phone] = true
	}
}
