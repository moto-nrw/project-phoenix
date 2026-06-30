package calendar

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	EventSourceAppointment = calModels.EventSourceAppointment
	EventSourceTimetable   = calModels.EventSourceTimetable

	maxCalendarWindowDays = 92
)

var (
	ErrInvalidRequest = errors.New("invalid calendar request")
	ErrForbidden      = errors.New("calendar access forbidden")
	ErrNotFound       = errors.New("calendar item not found")
)

type Service interface {
	ListMyStaffEvents(ctx context.Context, from, to timezone.Date) ([]Event, error)
	ListMyParentEvents(ctx context.Context, accountID int64, from, to timezone.Date) ([]Event, error)
	CreateStaffAppointment(ctx context.Context, req CreateAppointmentRequest) (*AppointmentDetail, error)
	GetStaffAppointmentOverview(ctx context.Context, appointmentID int64) (*AppointmentOverview, error)
	GetParentAppointmentOverview(ctx context.Context, accountID, appointmentID int64) (*AppointmentOverview, error)
	RespondToStaffInvitation(ctx context.Context, recipientID int64, status string) error
	RespondToParentInvitation(ctx context.Context, accountID, recipientID int64, status string) error
	RecipientOptions(ctx context.Context, query string, limit int) (*RecipientOptions, error)
}

type Config struct {
	AppointmentRepo      calModels.AppointmentRepository
	RecurrenceRepo       calModels.RecurrenceRuleRepository
	RecipientRepo        calModels.AppointmentRecipientRepository
	RecipientStudentRepo calModels.AppointmentRecipientStudentRepository
	TargetRepo           calModels.AppointmentTargetRepository
	OverrideRepo         calModels.AppointmentOccurrenceOverrideRepository
	StaffRepo            userModels.StaffRepository
	StudentRepo          userModels.StudentRepository
	GuardianProfileRepo  userModels.GuardianProfileRepository
	StudentGuardianRepo  userModels.StudentGuardianRepository
	ChildRepo            parentModels.ChildRepository
	GroupRepo            educationModels.GroupRepository
	InstanceStaffRepo    scheduleModels.InstanceStaffRepository
	InstanceStudentRepo  scheduleModels.InstanceStudentRepository
	ActivityInstanceRepo scheduleModels.ActivityInstanceRepository
	UserContext          usercontext.UserContextService
	DB                   *bun.DB
}

type service struct {
	cfg Config
}

func NewService(cfg Config) Service {
	return &service{cfg: cfg}
}

type Event struct {
	ID               string  `json:"id"`
	Source           string  `json:"source"`
	AppointmentID    *int64  `json:"appointment_id,omitempty"`
	OccurrenceDate   *string `json:"occurrence_date,omitempty"`
	TimetableID      *int64  `json:"timetable_id,omitempty"`
	StudentID        *int64  `json:"student_id,omitempty"`
	StudentName      *string `json:"student_name,omitempty"`
	TenantID         *int64  `json:"tenant_id,omitempty"`
	SchoolName       *string `json:"school_name,omitempty"`
	Title            string  `json:"title"`
	Description      *string `json:"description,omitempty"`
	Location         *string `json:"location,omitempty"`
	StartDate        string  `json:"start_date"`
	EndDate          string  `json:"end_date"`
	StartTime        string  `json:"start_time"`
	EndTime          string  `json:"end_time"`
	AllDay           bool    `json:"all_day"`
	DeliveryMode     *string `json:"delivery_mode,omitempty"`
	ResponseStatus   *string `json:"response_status,omitempty"`
	RecipientID      *int64  `json:"recipient_id,omitempty"`
	OrganizerStaffID *int64  `json:"organizer_staff_id,omitempty"`
	CanRespond       bool    `json:"can_respond"`
	CanEdit          bool    `json:"can_edit"`
	CanViewOverview  bool    `json:"can_view_overview"`
}

type AppointmentDetail struct {
	Appointment *calModels.Appointment            `json:"appointment"`
	Recurrence  *calModels.RecurrenceRule         `json:"recurrence,omitempty"`
	Recipients  []*calModels.AppointmentRecipient `json:"recipients"`
	Targets     []*calModels.AppointmentTarget    `json:"targets"`
}

type CreateAppointmentRequest struct {
	Title              string              `json:"title"`
	Description        *string             `json:"description,omitempty"`
	Location           *string             `json:"location,omitempty"`
	StartDate          timezone.Date       `json:"start_date"`
	EndDate            timezone.Date       `json:"end_date"`
	StartTime          time.Time           `json:"start_time"`
	EndTime            time.Time           `json:"end_time"`
	AllDay             bool                `json:"all_day"`
	DeliveryMode       string              `json:"delivery_mode"`
	OverviewVisibility string              `json:"overview_visibility"`
	Recurrence         *RecurrenceRequest  `json:"recurrence,omitempty"`
	Targets            []AppointmentTarget `json:"targets"`
}

type AppointmentOverview struct {
	AppointmentID      int64                 `json:"appointment_id"`
	DeliveryMode       string                `json:"delivery_mode"`
	OverviewVisibility string                `json:"overview_visibility"`
	Attendees          []AppointmentAttendee `json:"attendees"`
}

type AppointmentAttendee struct {
	RecipientID   int64      `json:"recipient_id"`
	RecipientType string     `json:"recipient_type"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	RespondedAt   *time.Time `json:"responded_at,omitempty"`
}

type RecurrenceRequest struct {
	Frequency       string         `json:"frequency"`
	IntervalCount   int            `json:"interval_count"`
	Weekdays        []string       `json:"weekdays,omitempty"`
	MonthDays       []int          `json:"month_days,omitempty"`
	EndsOn          *timezone.Date `json:"ends_on,omitempty"`
	OccurrenceCount *int           `json:"occurrence_count,omitempty"`
}

type AppointmentTarget struct {
	Type  string  `json:"type"`
	ID    *int64  `json:"id,omitempty"`
	Value *string `json:"value,omitempty"`
}

type RecipientOptions struct {
	Staff    []StaffOption   `json:"staff"`
	Parents  []ParentOption  `json:"parents"`
	Groups   []GroupOption   `json:"groups"`
	Classes  []string        `json:"classes"`
	Students []StudentOption `json:"students"`
}

type StaffOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ParentOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type GroupOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type StudentOption struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	SchoolClass string `json:"school_class,omitempty"`
	GroupID     *int64 `json:"group_id,omitempty"`
}

func (s *service) ListMyStaffEvents(ctx context.Context, from, to timezone.Date) ([]Event, error) {
	if err := validateWindow(from, to); err != nil {
		return nil, err
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}

	appointments, err := s.cfg.AppointmentRepo.ListVisibleForStaff(ctx, staff.ID, from, to)
	if err != nil {
		return nil, err
	}
	appointmentEvents, err := s.expandAppointmentEvents(ctx, appointments, staff.ID, from, to)
	if err != nil {
		return nil, err
	}
	timetableEvents, err := s.staffTimetableEvents(ctx, staff.ID, from, to)
	if err != nil {
		return nil, err
	}

	events := append(appointmentEvents, timetableEvents...)
	sortEvents(events)
	return events, nil
}

func (s *service) ListMyParentEvents(ctx context.Context, accountID int64, from, to timezone.Date) ([]Event, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if err := validateWindow(from, to); err != nil {
		return nil, err
	}

	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return nil, err
	}
	childrenByTenant := groupChildrenByTenant(children)
	events := []Event{}
	for tenantID, tenantChildren := range childrenByTenant {
		tenantID := tenantID
		tenantChildren := tenantChildren
		if err := tenant.WithTenantTx(ctx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
			appointments, err := s.cfg.AppointmentRepo.ListVisibleForGuardianProfiles(txCtx, guardianProfileIDs, from, to)
			if err != nil {
				return err
			}
			appointmentEvents, err := s.expandGuardianAppointmentEvents(txCtx, appointments, guardianProfileIDs, from, to)
			if err != nil {
				return err
			}
			schoolName := ""
			if len(tenantChildren) > 0 {
				schoolName = tenantChildren[0].SchoolName
			}
			for i := range appointmentEvents {
				appointmentEvents[i].TenantID = &tenantID
				if schoolName != "" {
					appointmentEvents[i].SchoolName = &schoolName
				}
			}
			timetableEvents, err := s.parentTimetableEvents(txCtx, tenantChildren, from, to)
			if err != nil {
				return err
			}
			events = append(events, appointmentEvents...)
			events = append(events, timetableEvents...)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	sortEvents(events)
	return events, nil
}

func (s *service) CreateStaffAppointment(ctx context.Context, req CreateAppointmentRequest) (*AppointmentDetail, error) {
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	if req.DeliveryMode == "" {
		req.DeliveryMode = calModels.DeliveryModeRSVPRequired
	}
	if req.OverviewVisibility == "" {
		req.OverviewVisibility = calModels.OverviewVisibilityOrganizer
	}
	if req.EndDate.IsZero() {
		req.EndDate = req.StartDate
	}

	appointment := &calModels.Appointment{
		OrganizerStaffID:   staff.ID,
		Title:              req.Title,
		Description:        req.Description,
		Location:           req.Location,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		StartTime:          timezone.WallClock(req.StartTime),
		EndTime:            timezone.WallClock(req.EndTime),
		AllDay:             req.AllDay,
		DeliveryMode:       req.DeliveryMode,
		OverviewVisibility: req.OverviewVisibility,
	}
	if err := appointment.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	recipients, recipientStudents, targets, err := s.resolveTargets(ctx, appointment.DeliveryMode, req.Targets)
	if err != nil {
		return nil, err
	}

	if err := s.cfg.AppointmentRepo.Create(ctx, appointment); err != nil {
		return nil, err
	}

	var recurrence *calModels.RecurrenceRule
	if req.Recurrence != nil {
		recurrence = &calModels.RecurrenceRule{
			AppointmentID:   appointment.ID,
			Frequency:       req.Recurrence.Frequency,
			IntervalCount:   req.Recurrence.IntervalCount,
			Weekdays:        normalizeWeekdays(req.Recurrence.Weekdays),
			MonthDays:       req.Recurrence.MonthDays,
			EndsOn:          req.Recurrence.EndsOn,
			OccurrenceCount: req.Recurrence.OccurrenceCount,
		}
		if recurrence.IntervalCount == 0 {
			recurrence.IntervalCount = 1
		}
		if err := recurrence.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
		if err := s.cfg.RecurrenceRepo.Create(ctx, recurrence); err != nil {
			return nil, err
		}
	}

	for _, recipient := range recipients {
		recipient.AppointmentID = appointment.ID
	}
	if err := s.cfg.RecipientRepo.CreateMany(ctx, recipients); err != nil {
		return nil, err
	}
	recipientIDByKey := make(map[string]int64, len(recipients))
	for _, recipient := range recipients {
		recipientIDByKey[recipientKey(recipient.RecipientType, recipient.StaffID, recipient.GuardianProfileID)] = recipient.ID
	}
	for _, link := range recipientStudents {
		key := recipientKey(calModels.RecipientTypeGuardianProfile, nil, &link.RecipientID)
		if recipientID, ok := recipientIDByKey[key]; ok {
			link.RecipientID = recipientID
		}
	}
	if err := s.cfg.RecipientStudentRepo.CreateMany(ctx, recipientStudents); err != nil {
		return nil, err
	}

	for _, target := range targets {
		target.AppointmentID = appointment.ID
	}
	if err := s.cfg.TargetRepo.ReplaceForAppointment(ctx, appointment.ID, targets); err != nil {
		return nil, err
	}

	return &AppointmentDetail{
		Appointment: appointment,
		Recurrence:  recurrence,
		Recipients:  recipients,
		Targets:     targets,
	}, nil
}

func (s *service) GetStaffAppointmentOverview(ctx context.Context, appointmentID int64) (*AppointmentOverview, error) {
	if appointmentID <= 0 {
		return nil, fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	appointment, err := s.cfg.AppointmentRepo.FindByID(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appointment == nil {
		return nil, ErrNotFound
	}
	recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	_, recipientID := staffRecipientStatus(recipients, staff.ID)
	if appointment.OrganizerStaffID != staff.ID && recipientID == nil {
		return nil, ErrNotFound
	}
	if !canStaffViewOverview(appointment, staff.ID, recipientID != nil) {
		return nil, ErrForbidden
	}
	return s.buildAppointmentOverview(ctx, appointment, recipients)
}

func (s *service) GetParentAppointmentOverview(ctx context.Context, accountID, appointmentID int64) (*AppointmentOverview, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if appointmentID <= 0 {
		return nil, fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return nil, err
	}
	childrenByTenant := groupChildrenByTenant(children)
	var foundForbidden bool
	for tenantID, tenantChildren := range childrenByTenant {
		tenantID := tenantID
		tenantChildren := tenantChildren
		var overview *AppointmentOverview
		err := tenant.WithTenantTx(ctx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			appointment, err := s.cfg.AppointmentRepo.FindByID(txCtx, appointmentID)
			if err != nil {
				return err
			}
			if appointment == nil {
				return nil
			}
			recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(txCtx, appointment.ID)
			if err != nil {
				return err
			}
			guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
			_, recipientID := guardianRecipientStatus(recipients, guardianProfileIDs)
			if recipientID == nil {
				return nil
			}
			if !canParentViewOverview(appointment, true) {
				foundForbidden = true
				return nil
			}
			overview, err = s.buildAppointmentOverview(txCtx, appointment, recipients)
			return err
		})
		if err != nil {
			return nil, err
		}
		if overview != nil {
			return overview, nil
		}
	}
	if foundForbidden {
		return nil, ErrForbidden
	}
	return nil, ErrNotFound
}

func (s *service) RespondToStaffInvitation(ctx context.Context, recipientID int64, status string) error {
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	if status != calModels.ResponseStatusAccepted && status != calModels.ResponseStatusDeclined {
		return fmt.Errorf("%w: response status must be accepted or declined", ErrInvalidRequest)
	}

	recipient, err := s.cfg.RecipientRepo.FindByID(ctx, recipientID)
	if err != nil {
		return err
	}
	if recipient == nil || recipient.StaffID == nil || *recipient.StaffID != staff.ID {
		return ErrNotFound
	}
	if recipient.Status == calModels.ResponseStatusInfo {
		return fmt.Errorf("%w: informational appointments cannot be answered", ErrInvalidRequest)
	}
	return s.cfg.RecipientRepo.UpdateResponse(ctx, recipientID, status)
}

func (s *service) RespondToParentInvitation(ctx context.Context, accountID, recipientID int64, status string) error {
	if accountID <= 0 {
		return fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if status != calModels.ResponseStatusAccepted && status != calModels.ResponseStatusDeclined {
		return fmt.Errorf("%w: response status must be accepted or declined", ErrInvalidRequest)
	}

	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return err
	}
	for tenantID, tenantChildren := range groupChildrenByTenant(children) {
		allowedProfiles := int64Set(distinctGuardianProfileIDs(tenantChildren))
		var updated bool
		if err := tenant.WithTenantTx(ctx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			recipient, err := s.cfg.RecipientRepo.FindByID(txCtx, recipientID)
			if err != nil {
				return err
			}
			if recipient == nil || recipient.GuardianProfileID == nil {
				return nil
			}
			if _, ok := allowedProfiles[*recipient.GuardianProfileID]; !ok {
				return nil
			}
			if recipient.Status == calModels.ResponseStatusInfo {
				return fmt.Errorf("%w: informational appointments cannot be answered", ErrInvalidRequest)
			}
			if err := s.cfg.RecipientRepo.UpdateResponse(txCtx, recipientID, status); err != nil {
				return err
			}
			updated = true
			return nil
		}); err != nil {
			return err
		}
		if updated {
			return nil
		}
	}
	return ErrNotFound
}

func (s *service) RecipientOptions(ctx context.Context, query string, limit int) (*RecipientOptions, error) {
	if limit <= 0 || limit > 50 {
		limit = 25
	}
	query = strings.ToLower(strings.TrimSpace(query))

	staffRows, err := s.cfg.StaffRepo.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	staffOptions := make([]StaffOption, 0, min(limit, len(staffRows)))
	for _, row := range staffRows {
		name := staffName(row)
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			staffOptions = append(staffOptions, StaffOption{ID: row.ID, Name: name})
			if len(staffOptions) >= limit {
				break
			}
		}
	}

	parents, err := s.cfg.GuardianProfileRepo.SearchByText(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	parentOptions := make([]ParentOption, 0, len(parents))
	for _, parent := range parents {
		parentOptions = append(parentOptions, ParentOption{ID: parent.ID, Name: guardianName(parent)})
	}

	groups, err := s.cfg.GroupRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	groupOptions := make([]GroupOption, 0, min(limit, len(groups)))
	for _, group := range groups {
		if query == "" || strings.Contains(strings.ToLower(group.Name), query) {
			groupOptions = append(groupOptions, GroupOption{ID: group.ID, Name: group.Name})
			if len(groupOptions) >= limit {
				break
			}
		}
	}

	classes, err := s.cfg.StudentRepo.ListSchoolClasses(ctx)
	if err != nil {
		return nil, err
	}
	classOptions := make([]string, 0, min(limit, len(classes)))
	for _, className := range classes {
		if query == "" || strings.Contains(strings.ToLower(className), query) {
			classOptions = append(classOptions, className)
			if len(classOptions) >= limit {
				break
			}
		}
	}

	students, err := s.cfg.StudentRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	studentOptions := make([]StudentOption, 0, min(limit, len(students)))
	for _, student := range students {
		name := studentDisplayName(student)
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			studentOptions = append(studentOptions, StudentOption{
				ID:          student.ID,
				Name:        name,
				SchoolClass: student.SchoolClass,
				GroupID:     student.GroupID,
			})
			if len(studentOptions) >= limit {
				break
			}
		}
	}

	return &RecipientOptions{
		Staff:    staffOptions,
		Parents:  parentOptions,
		Groups:   groupOptions,
		Classes:  classOptions,
		Students: studentOptions,
	}, nil
}

func validateWindow(from, to timezone.Date) error {
	if from.IsZero() || to.IsZero() {
		return fmt.Errorf("%w: from and to are required", ErrInvalidRequest)
	}
	if to.Before(from) {
		return fmt.Errorf("%w: to must be on or after from", ErrInvalidRequest)
	}
	if from.DaysUntil(to)+1 > maxCalendarWindowDays {
		return fmt.Errorf("%w: range cannot exceed %d days", ErrInvalidRequest, maxCalendarWindowDays)
	}
	return nil
}

func (s *service) expandAppointmentEvents(ctx context.Context, appointments []*calModels.Appointment, staffID int64, from, to timezone.Date) ([]Event, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	recurrenceByAppointment := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	overrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsInRange(ctx, ids, from, to)
	if err != nil {
		return nil, err
	}
	overrideByAppointmentDate := make(map[string]*calModels.AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overrideByAppointmentDate[fmt.Sprintf("%d:%s", override.AppointmentID, override.OccurrenceDate.String())] = override
	}

	events := make([]Event, 0, len(appointments))
	for _, appointment := range appointments {
		recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointment.ID)
		if err != nil {
			return nil, err
		}
		status, recipientID := staffRecipientStatus(recipients, staffID)
		recurrence := recurrenceByAppointment[appointment.ID]
		if recurrence == nil {
			if !dateRangesOverlap(appointment.StartDate, appointment.EndDate, from, to) {
				continue
			}
			event := appointmentEvent(appointment, appointment.StartDate, status, recipientID, staffID)
			events = append(events, event)
			continue
		}
		for _, occurrence := range expandOccurrences(appointment, recurrence, from, to) {
			override := overrideByAppointmentDate[fmt.Sprintf("%d:%s", appointment.ID, occurrence.String())]
			if override != nil && override.Cancelled {
				continue
			}
			event := appointmentEvent(appointment, occurrence, status, recipientID, staffID)
			applyOverride(&event, override)
			events = append(events, event)
		}
	}
	return events, nil
}

func (s *service) expandGuardianAppointmentEvents(ctx context.Context, appointments []*calModels.Appointment, guardianProfileIDs []int64, from, to timezone.Date) ([]Event, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	recurrenceByAppointment := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	overrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsInRange(ctx, ids, from, to)
	if err != nil {
		return nil, err
	}
	overrideByAppointmentDate := make(map[string]*calModels.AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overrideByAppointmentDate[fmt.Sprintf("%d:%s", override.AppointmentID, override.OccurrenceDate.String())] = override
	}

	events := make([]Event, 0, len(appointments))
	for _, appointment := range appointments {
		recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointment.ID)
		if err != nil {
			return nil, err
		}
		status, recipientID := guardianRecipientStatus(recipients, guardianProfileIDs)
		recurrence := recurrenceByAppointment[appointment.ID]
		if recurrence == nil {
			if !dateRangesOverlap(appointment.StartDate, appointment.EndDate, from, to) {
				continue
			}
			event := appointmentEvent(appointment, appointment.StartDate, status, recipientID, 0)
			event.CanViewOverview = canParentViewOverview(appointment, recipientID != nil)
			events = append(events, event)
			continue
		}
		for _, occurrence := range expandOccurrences(appointment, recurrence, from, to) {
			override := overrideByAppointmentDate[fmt.Sprintf("%d:%s", appointment.ID, occurrence.String())]
			if override != nil && override.Cancelled {
				continue
			}
			event := appointmentEvent(appointment, occurrence, status, recipientID, 0)
			event.CanViewOverview = canParentViewOverview(appointment, recipientID != nil)
			applyOverride(&event, override)
			events = append(events, event)
		}
	}
	return events, nil
}

func (s *service) staffTimetableEvents(ctx context.Context, staffID int64, from, to timezone.Date) ([]Event, error) {
	events := []Event{}
	seen := make(map[int64]struct{})
	for d := from; !d.After(to); d = d.AddDays(1) {
		assignments, err := s.cfg.InstanceStaffRepo.FindByStaffAndDate(ctx, staffID, d)
		if err != nil {
			return nil, err
		}
		for _, assignment := range assignments {
			if _, ok := seen[assignment.InstanceID]; ok {
				continue
			}
			seen[assignment.InstanceID] = struct{}{}
			instance, err := s.cfg.ActivityInstanceRepo.FindByID(ctx, assignment.InstanceID)
			if err != nil {
				return nil, err
			}
			id := instance.ID
			events = append(events, Event{
				ID:          fmt.Sprintf("timetable:%d", instance.ID),
				Source:      EventSourceTimetable,
				TimetableID: &id,
				Title:       instance.Title,
				Description: instance.Description,
				StartDate:   instance.Date.String(),
				EndDate:     instance.Date.String(),
				StartTime:   formatClock(instance.StartTime),
				EndTime:     formatClock(instance.EndTime),
				AllDay:      false,
			})
		}
	}
	return events, nil
}

func (s *service) parentTimetableEvents(ctx context.Context, children []*parentModels.ChildSummary, from, to timezone.Date) ([]Event, error) {
	events := []Event{}
	seen := make(map[string]struct{})
	for _, child := range children {
		rows, err := s.cfg.InstanceStudentRepo.FindByStudentAndDateRange(ctx, child.StudentID, from, to)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			key := fmt.Sprintf("%d:%d", child.StudentID, row.InstanceID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			instance, err := s.cfg.ActivityInstanceRepo.FindByID(ctx, row.InstanceID)
			if err != nil {
				return nil, err
			}
			timetableID := instance.ID
			studentID := child.StudentID
			tenantID := child.TenantID
			studentName := strings.TrimSpace(child.FirstName + " " + child.LastName)
			schoolName := child.SchoolName
			events = append(events, Event{
				ID:          fmt.Sprintf("timetable:%d:%d", child.StudentID, instance.ID),
				Source:      EventSourceTimetable,
				TimetableID: &timetableID,
				StudentID:   &studentID,
				StudentName: &studentName,
				TenantID:    &tenantID,
				SchoolName:  &schoolName,
				Title:       instance.Title,
				Description: instance.Description,
				StartDate:   instance.Date.String(),
				EndDate:     instance.Date.String(),
				StartTime:   formatClock(instance.StartTime),
				EndTime:     formatClock(instance.EndTime),
				AllDay:      false,
			})
		}
	}
	return events, nil
}

func (s *service) resolveTargets(ctx context.Context, deliveryMode string, targets []AppointmentTarget) ([]*calModels.AppointmentRecipient, []*calModels.AppointmentRecipientStudent, []*calModels.AppointmentTarget, error) {
	status := calModels.ResponseStatusPending
	if deliveryMode == calModels.DeliveryModeInformational {
		status = calModels.ResponseStatusInfo
	}

	staffIDs := map[int64]struct{}{}
	guardianStudents := map[int64]map[int64]struct{}{}
	targetRows := make([]*calModels.AppointmentTarget, 0, len(targets))
	addGuardian := func(guardianProfileID int64, studentID *int64) {
		if guardianProfileID <= 0 {
			return
		}
		if _, ok := guardianStudents[guardianProfileID]; !ok {
			guardianStudents[guardianProfileID] = map[int64]struct{}{}
		}
		if studentID != nil && *studentID > 0 {
			guardianStudents[guardianProfileID][*studentID] = struct{}{}
		}
	}
	addStudentGuardians := func(studentID int64) error {
		links, err := s.cfg.StudentGuardianRepo.FindByStudentID(ctx, studentID)
		if err != nil {
			return err
		}
		for _, link := range links {
			if authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) {
				addGuardian(link.GuardianProfileID, &studentID)
			}
		}
		return nil
	}

	for _, target := range targets {
		targetRows = append(targetRows, &calModels.AppointmentTarget{
			TargetType:  target.Type,
			TargetID:    target.ID,
			TargetValue: target.Value,
		})
		switch target.Type {
		case calModels.TargetTypeAllStaff:
			staffRows, err := s.cfg.StaffRepo.ListAllWithPerson(ctx)
			if err != nil {
				return nil, nil, nil, err
			}
			for _, staff := range staffRows {
				staffIDs[staff.ID] = struct{}{}
			}
		case calModels.TargetTypeStaff:
			if target.ID == nil || *target.ID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: staff target requires id", ErrInvalidRequest)
			}
			staffIDs[*target.ID] = struct{}{}
		case calModels.TargetTypeGuardianProfile:
			if target.ID == nil || *target.ID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: guardian target requires id", ErrInvalidRequest)
			}
			addGuardian(*target.ID, nil)
		case calModels.TargetTypeParentsByStudent:
			if target.ID == nil || *target.ID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: student target requires id", ErrInvalidRequest)
			}
			if err := addStudentGuardians(*target.ID); err != nil {
				return nil, nil, nil, err
			}
		case calModels.TargetTypeParentsByGroup:
			if target.ID == nil || *target.ID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: group target requires id", ErrInvalidRequest)
			}
			students, err := s.cfg.StudentRepo.FindByGroupID(ctx, *target.ID)
			if err != nil {
				return nil, nil, nil, err
			}
			for _, student := range students {
				if err := addStudentGuardians(student.ID); err != nil {
					return nil, nil, nil, err
				}
			}
		case calModels.TargetTypeParentsByClass:
			if target.Value == nil || strings.TrimSpace(*target.Value) == "" {
				return nil, nil, nil, fmt.Errorf("%w: class target requires value", ErrInvalidRequest)
			}
			students, err := s.cfg.StudentRepo.FindBySchoolClass(ctx, strings.TrimSpace(*target.Value))
			if err != nil {
				return nil, nil, nil, err
			}
			for _, student := range students {
				if err := addStudentGuardians(student.ID); err != nil {
					return nil, nil, nil, err
				}
			}
		default:
			return nil, nil, nil, fmt.Errorf("%w: unknown target type %q", ErrInvalidRequest, target.Type)
		}
	}

	recipients := make([]*calModels.AppointmentRecipient, 0, len(staffIDs)+len(guardianStudents))
	for staffID := range staffIDs {
		id := staffID
		recipients = append(recipients, &calModels.AppointmentRecipient{
			RecipientType: calModels.RecipientTypeStaff,
			StaffID:       &id,
			Status:        status,
		})
	}
	recipientStudents := []*calModels.AppointmentRecipientStudent{}
	for guardianProfileID, studentIDs := range guardianStudents {
		id := guardianProfileID
		recipients = append(recipients, &calModels.AppointmentRecipient{
			RecipientType:     calModels.RecipientTypeGuardianProfile,
			GuardianProfileID: &id,
			Status:            status,
		})
		for studentID := range studentIDs {
			recipientStudents = append(recipientStudents, &calModels.AppointmentRecipientStudent{
				RecipientID: guardianProfileID,
				StudentID:   studentID,
			})
		}
	}
	return recipients, recipientStudents, targetRows, nil
}

func (s *service) buildAppointmentOverview(ctx context.Context, appointment *calModels.Appointment, recipients []*calModels.AppointmentRecipient) (*AppointmentOverview, error) {
	staffIDs := make([]int64, 0)
	guardianIDs := make([]int64, 0)
	for _, recipient := range recipients {
		if recipient.StaffID != nil {
			staffIDs = append(staffIDs, *recipient.StaffID)
		}
		if recipient.GuardianProfileID != nil {
			guardianIDs = append(guardianIDs, *recipient.GuardianProfileID)
		}
	}

	staffByID, err := s.cfg.StaffRepo.FindWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	guardiansByID, err := s.cfg.GuardianProfileRepo.FindByIDs(ctx, guardianIDs)
	if err != nil {
		return nil, err
	}

	attendees := make([]AppointmentAttendee, 0, len(recipients))
	for _, recipient := range recipients {
		name := ""
		if recipient.StaffID != nil {
			name = staffName(staffByID[*recipient.StaffID])
		}
		if recipient.GuardianProfileID != nil {
			name = guardianName(guardiansByID[*recipient.GuardianProfileID])
		}
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("Empfänger %d", recipient.ID)
		}
		attendees = append(attendees, AppointmentAttendee{
			RecipientID:   recipient.ID,
			RecipientType: recipient.RecipientType,
			Name:          name,
			Status:        recipient.Status,
			RespondedAt:   recipient.RespondedAt,
		})
	}
	sort.SliceStable(attendees, func(i, j int) bool {
		if attendees[i].RecipientType != attendees[j].RecipientType {
			return attendees[i].RecipientType < attendees[j].RecipientType
		}
		return strings.ToLower(attendees[i].Name) < strings.ToLower(attendees[j].Name)
	})

	return &AppointmentOverview{
		AppointmentID:      appointment.ID,
		DeliveryMode:       appointment.DeliveryMode,
		OverviewVisibility: appointment.OverviewVisibility,
		Attendees:          attendees,
	}, nil
}

func canStaffViewOverview(appointment *calModels.Appointment, staffID int64, isRecipient bool) bool {
	if appointment == nil {
		return false
	}
	if appointment.OrganizerStaffID == staffID {
		return true
	}
	switch appointment.OverviewVisibility {
	case calModels.OverviewVisibilityStaff, calModels.OverviewVisibilityAll:
		return isRecipient
	default:
		return false
	}
}

func canParentViewOverview(appointment *calModels.Appointment, isRecipient bool) bool {
	return appointment != nil && isRecipient && appointment.OverviewVisibility == calModels.OverviewVisibilityAll
}

func appointmentEvent(appointment *calModels.Appointment, occurrenceDate timezone.Date, responseStatus *string, recipientID *int64, staffID int64) Event {
	appointmentID := appointment.ID
	deliveryMode := appointment.DeliveryMode
	occurrence := occurrenceDate.String()
	endDate := occurrenceDate.AddDays(appointment.StartDate.DaysUntil(appointment.EndDate))
	isStaffRecipient := recipientID != nil
	return Event{
		ID:               fmt.Sprintf("appointment:%d:%s", appointment.ID, occurrence),
		Source:           EventSourceAppointment,
		AppointmentID:    &appointmentID,
		OccurrenceDate:   &occurrence,
		Title:            appointment.Title,
		Description:      appointment.Description,
		Location:         appointment.Location,
		StartDate:        occurrenceDate.String(),
		EndDate:          endDate.String(),
		StartTime:        formatClock(appointment.StartTime),
		EndTime:          formatClock(appointment.EndTime),
		AllDay:           appointment.AllDay,
		DeliveryMode:     &deliveryMode,
		ResponseStatus:   responseStatus,
		RecipientID:      recipientID,
		OrganizerStaffID: &appointment.OrganizerStaffID,
		CanRespond:       recipientID != nil && responseStatus != nil && *responseStatus == calModels.ResponseStatusPending,
		CanEdit:          appointment.OrganizerStaffID == staffID,
		CanViewOverview:  canStaffViewOverview(appointment, staffID, isStaffRecipient),
	}
}

func applyOverride(event *Event, override *calModels.AppointmentOccurrenceOverride) {
	if override == nil {
		return
	}
	if override.Title != nil {
		event.Title = *override.Title
	}
	if override.Description != nil {
		event.Description = override.Description
	}
	if override.Location != nil {
		event.Location = override.Location
	}
	if override.StartDate != nil {
		event.StartDate = override.StartDate.String()
	}
	if override.EndDate != nil {
		event.EndDate = override.EndDate.String()
	}
	if override.StartTime != nil {
		event.StartTime = formatClock(*override.StartTime)
	}
	if override.EndTime != nil {
		event.EndTime = formatClock(*override.EndTime)
	}
	if override.AllDay != nil {
		event.AllDay = *override.AllDay
	}
}

func staffRecipientStatus(recipients []*calModels.AppointmentRecipient, staffID int64) (*string, *int64) {
	for _, recipient := range recipients {
		if recipient.StaffID != nil && *recipient.StaffID == staffID {
			status := recipient.Status
			id := recipient.ID
			return &status, &id
		}
	}
	return nil, nil
}

func guardianRecipientStatus(recipients []*calModels.AppointmentRecipient, guardianProfileIDs []int64) (*string, *int64) {
	allowed := int64Set(guardianProfileIDs)
	for _, recipient := range recipients {
		if recipient.GuardianProfileID == nil {
			continue
		}
		if _, ok := allowed[*recipient.GuardianProfileID]; ok {
			status := recipient.Status
			id := recipient.ID
			return &status, &id
		}
	}
	return nil, nil
}

func (s *service) parentChildren(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	var children []*parentModels.ChildSummary
	if err := tenant.WithAdminTx(ctx, s.cfg.DB, func(adminCtx context.Context, _ bun.Tx) error {
		rows, err := s.cfg.ChildRepo.ListByAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		children = rows
		return nil
	}); err != nil {
		return nil, err
	}
	return children, nil
}

func groupChildrenByTenant(children []*parentModels.ChildSummary) map[int64][]*parentModels.ChildSummary {
	out := make(map[int64][]*parentModels.ChildSummary)
	for _, child := range children {
		out[child.TenantID] = append(out[child.TenantID], child)
	}
	return out
}

func distinctGuardianProfileIDs(children []*parentModels.ChildSummary) []int64 {
	seen := map[int64]struct{}{}
	out := []int64{}
	for _, child := range children {
		if _, ok := seen[child.GuardianProfileID]; ok {
			continue
		}
		seen[child.GuardianProfileID] = struct{}{}
		out = append(out, child.GuardianProfileID)
	}
	return out
}

func int64Set(values []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func expandOccurrences(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, from, to timezone.Date) []timezone.Date {
	occurrences := []timezone.Date{}
	if rule.IntervalCount <= 0 {
		rule.IntervalCount = 1
	}
	count := 0
	for d := appointment.StartDate; !d.After(to); d = d.AddDays(1) {
		if rule.EndsOn != nil && d.After(*rule.EndsOn) {
			break
		}
		if matchesRule(appointment.StartDate, d, rule) {
			count++
			if rule.OccurrenceCount != nil && count > *rule.OccurrenceCount {
				break
			}
			endDate := d.AddDays(appointment.StartDate.DaysUntil(appointment.EndDate))
			if dateRangesOverlap(d, endDate, from, to) {
				occurrences = append(occurrences, d)
			}
		}
	}
	return occurrences
}

func matchesRule(start, candidate timezone.Date, rule *calModels.RecurrenceRule) bool {
	if candidate.Before(start) {
		return false
	}
	switch rule.Frequency {
	case calModels.RecurrenceFrequencyDaily:
		return start.DaysUntil(candidate)%rule.IntervalCount == 0
	case calModels.RecurrenceFrequencyWeekly:
		weeks := start.DaysUntil(candidate) / 7
		if weeks%rule.IntervalCount != 0 {
			return false
		}
		weekdays := normalizeWeekdays(rule.Weekdays)
		if len(weekdays) == 0 {
			return candidate.Weekday() == start.Weekday()
		}
		return containsString(weekdays, strings.ToLower(candidate.Weekday().String()))
	case calModels.RecurrenceFrequencyMonthly:
		months := (candidate.Year-start.Year)*12 + int(candidate.Month-start.Month)
		if months < 0 || months%rule.IntervalCount != 0 {
			return false
		}
		if len(rule.MonthDays) == 0 {
			return candidate.Day == start.Day
		}
		for _, day := range rule.MonthDays {
			if candidate.Day == day {
				return true
			}
		}
		return false
	case calModels.RecurrenceFrequencyYearly:
		years := candidate.Year - start.Year
		return years >= 0 && years%rule.IntervalCount == 0 && candidate.Month == start.Month && candidate.Day == start.Day
	default:
		return false
	}
}

func normalizeWeekdays(days []string) []string {
	out := make([]string, 0, len(days))
	for _, day := range days {
		normalized := strings.ToLower(strings.TrimSpace(day))
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func dateRangesOverlap(aStart, aEnd, bStart, bEnd timezone.Date) bool {
	return !aEnd.Before(bStart) && !aStart.After(bEnd)
}

func sortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].StartDate != events[j].StartDate {
			return events[i].StartDate < events[j].StartDate
		}
		if events[i].StartTime != events[j].StartTime {
			return events[i].StartTime < events[j].StartTime
		}
		return events[i].ID < events[j].ID
	})
}

func formatClock(t time.Time) string {
	return timezone.WallClock(t).Format("15:04")
}

func staffName(staff *userModels.Staff) string {
	if staff == nil || staff.Person == nil {
		return ""
	}
	return strings.TrimSpace(staff.Person.FirstName + " " + staff.Person.LastName)
}

func guardianName(parent *userModels.GuardianProfile) string {
	if parent == nil {
		return ""
	}
	name := strings.TrimSpace(parent.FirstName + " " + parent.LastName)
	if name == "" && parent.Email != nil {
		return *parent.Email
	}
	return name
}

func studentDisplayName(student *userModels.Student) string {
	if student == nil {
		return ""
	}
	if student.Person == nil {
		return fmt.Sprintf("Kind %d", student.ID)
	}
	return strings.TrimSpace(student.Person.FirstName + " " + student.Person.LastName)
}

func recipientKey(recipientType string, staffID, guardianProfileID *int64) string {
	switch recipientType {
	case calModels.RecipientTypeStaff:
		if staffID != nil {
			return fmt.Sprintf("staff:%d", *staffID)
		}
	case calModels.RecipientTypeGuardianProfile:
		if guardianProfileID != nil {
			return fmt.Sprintf("guardian:%d", *guardianProfileID)
		}
	}
	return recipientType + ":0"
}
