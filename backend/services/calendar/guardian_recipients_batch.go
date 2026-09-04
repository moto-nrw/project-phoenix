package calendar

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type guardianRecipientReadSet struct {
	recipientsByAppointment map[int64][]*calModels.AppointmentRecipient
	profiles                map[int64]*userModels.GuardianProfile
	studentsByRecipient     map[int64][]int64
	allowed                 map[[2]int64]bool
}

func (s *service) reachableGuardianRecipientsByAppointment(ctx context.Context, appointmentIDs []int64) (map[int64]guardianRecipients, error) {
	readSet, err := s.loadGuardianRecipientReadSet(ctx, appointmentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]guardianRecipients, len(appointmentIDs))
	for _, appointmentID := range appointmentIDs {
		result[appointmentID] = buildGuardianRecipients(readSet.recipientsByAppointment[appointmentID], readSet)
	}
	return result, nil
}

func (s *service) loadGuardianRecipientReadSet(ctx context.Context, appointmentIDs []int64) (*guardianRecipientReadSet, error) {
	recipients, err := s.cfg.Appointments.FindAppointmentRecipientsByAppointmentIDs(ctx, appointmentIDs)
	if err != nil {
		return nil, err
	}
	readSet, guardianIDs, recipientIDs := indexGuardianRecipientRows(recipients)
	if len(guardianIDs) == 0 {
		return readSet, nil
	}
	readSet.profiles, err = s.cfg.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, guardianIDs)
	if err != nil {
		return nil, err
	}
	if s.cfg.StudentGuardianRepo == nil {
		return nil, errors.New("calendar: guardian permission repositories are required")
	}
	studentLinks, err := s.cfg.Appointments.FindAppointmentRecipientStudents(ctx, recipientIDs)
	if err != nil {
		return nil, err
	}
	studentIDs := indexRecipientStudents(readSet.studentsByRecipient, studentLinks)
	guardianLinks, err := s.cfg.StudentGuardianRepo.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	readSet.allowed = guardianPermissionSet(guardianLinks)
	return readSet, nil
}

func indexGuardianRecipientRows(recipients []*calModels.AppointmentRecipient) (*guardianRecipientReadSet, []int64, []int64) {
	readSet := &guardianRecipientReadSet{
		recipientsByAppointment: make(map[int64][]*calModels.AppointmentRecipient),
		profiles:                make(map[int64]*userModels.GuardianProfile),
		studentsByRecipient:     make(map[int64][]int64),
		allowed:                 make(map[[2]int64]bool),
	}
	guardianIDs := make([]int64, 0, len(recipients))
	recipientIDs := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		readSet.recipientsByAppointment[recipient.AppointmentID] = append(readSet.recipientsByAppointment[recipient.AppointmentID], recipient)
		if recipient.RecipientType == calModels.RecipientTypeGuardianProfile && recipient.GuardianProfileID != nil {
			guardianIDs = append(guardianIDs, *recipient.GuardianProfileID)
			recipientIDs = append(recipientIDs, recipient.ID)
		}
	}
	return readSet, guardianIDs, recipientIDs
}

func indexRecipientStudents(byRecipient map[int64][]int64, links []*calModels.AppointmentRecipientStudent) []int64 {
	studentIDs := make([]int64, 0, len(links))
	seen := make(map[int64]bool)
	for _, link := range links {
		byRecipient[link.RecipientID] = append(byRecipient[link.RecipientID], link.StudentID)
		appendDistinctID(&studentIDs, seen, link.StudentID)
	}
	return studentIDs
}

func guardianPermissionSet(links []*userModels.StudentGuardian) map[[2]int64]bool {
	allowed := make(map[[2]int64]bool, len(links))
	for _, link := range links {
		if authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) {
			allowed[[2]int64{link.GuardianProfileID, link.StudentID}] = true
		}
	}
	return allowed
}

func buildGuardianRecipients(recipients []*calModels.AppointmentRecipient, readSet *guardianRecipientReadSet) guardianRecipients {
	result := guardianRecipients{profiles: readSet.profiles, studentsByGuardian: make(map[int64][]int64)}
	for _, recipient := range recipients {
		if recipient.GuardianProfileID == nil {
			continue
		}
		guardianID := *recipient.GuardianProfileID
		profile := readSet.profiles[guardianID]
		if profile == nil || profile.AccountID == nil || *profile.AccountID <= 0 {
			continue
		}
		allowedStudents := make([]int64, 0, len(readSet.studentsByRecipient[recipient.ID]))
		for _, studentID := range readSet.studentsByRecipient[recipient.ID] {
			if readSet.allowed[[2]int64{guardianID, studentID}] {
				allowedStudents = append(allowedStudents, studentID)
			}
		}
		if len(allowedStudents) > 0 {
			result.guardianIDs = append(result.guardianIDs, guardianID)
			result.studentsByGuardian[guardianID] = append(result.studentsByGuardian[guardianID], allowedStudents...)
		}
	}
	return result
}
