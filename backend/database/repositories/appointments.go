package repositories

import (
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	"github.com/uptrace/bun"
)

func NewAppointments(db *bun.DB) (appointments.Capability, error) {
	return appointmentsCompose.New(appointmentsCompose.Dependencies{
		DB:      db,
		Observe: func(appointmentsCompose.Observation) {},
	})
}

func (f *Factory) BindAppointments(capability appointments.Capability) {
	if capability == nil {
		panic("repository factory: appointments capability is required")
	}
	if f.appointmentsBound {
		return
	}
	f.appointmentsBound = true
	f.bindAppointments(capability)
}

func (f *Factory) Appointments() appointments.Capability { return f.appointments }

func (f *Factory) bindAppointments(capability appointments.Capability) {
	f.appointments = capability
}
