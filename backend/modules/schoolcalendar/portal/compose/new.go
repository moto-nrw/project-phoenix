// Package compose constructs the calendar portal from consumer-owned ports.
// It does not construct a service or repository factory.
package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/adapters/tokens"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
)

type Config = application.Config
type EmailConfig = application.EmailConfig
type ReminderEffects = application.ReminderEffects
type GuardianNotificationAudience = application.GuardianNotificationAudience
type GuardianNotificationProfile = application.GuardianNotificationProfile
type Application interface {
	portal.FullService
	ReminderEffects() ReminderEffects
	GuardianNotificationAudiences(context.Context, []int64) (map[int64]GuardianNotificationAudience, error)
}

func New(cfg Config) Application {
	cfg.NewFeedToken = tokens.New
	cfg.Digest = tokens.Digest
	return application.NewService(cfg)
}

func NewAppointmentRenderer(cfg EmailConfig) func(context.Context, *EmailOutbox) (*Message, error) {
	return application.NewAppointmentRenderer(cfg)
}

type Person = ports.Person
type Staff = ports.Staff
type Student = ports.Student
type GuardianProfile = ports.GuardianProfile
type StudentGuardian = ports.StudentGuardian
type ChildSummary = ports.ChildSummary
type Group = ports.Group
type School = ports.School
type Account = ports.Account
type FeedOwner = ports.FeedOwner
type Room = ports.Room
type InstanceStaff = ports.InstanceStaff
type ActivityInstance = ports.ActivityInstance
type StaffShift = ports.StaffShift
type ShiftType = ports.ShiftType
type EnqueueRequest = ports.EnqueueRequest
type EmailOutbox = ports.EmailOutbox
type Message = ports.Message
type Notification = ports.Event
