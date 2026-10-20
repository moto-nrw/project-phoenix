package enrollment

import (
	"context"
	"sort"
	"time"
)

// OfferingBookings are the booked offering intervals of request children.
// Care Plan keeps them; the root binds this port to its booking store
// (#4012, ADR 0053 point 3).
type OfferingBookings interface {
	CareOfferingBookingHistory(ctx context.Context, childIDs []int64) ([]OfferingBooking, error)
	CareOfferingBookingsAtDates(ctx context.Context, dates map[int64]Date) ([]OfferingBooking, error)
	CareOfferingBookingsForOfferings(ctx context.Context, offeringIDs []int64) ([]OfferingBooking, error)
	CareOfferingBookingsForPhase(ctx context.Context, phaseID int64) ([]OfferingBooking, error)
}

// OfferingBooking is one booked offering interval of a request child. SelectedDays
// are the effective days Care Plan derives from the manual and the automatic
// selection.
type OfferingBooking struct {
	ID                    int64
	TenantID              int64
	RequestChildID        int64
	CareOfferingID        int64
	SelectedDays          []string
	ManualSelectedDays    []string
	AutomaticSelectedDays []string
	ValidFrom             *Date
	ValidUntil            *Date
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// BookingProjection joins the Enrollment owner's application identity with
// the effective bookings of its children: the selections, capacity peaks and
// approved bookings the flows read. It neither reads nor writes a
// compatibility view.
type BookingProjection struct {
	Owner
	bookings OfferingBookings
}

// NewBookingProjection binds the projection to the owner and the bookings.
func NewBookingProjection(owner Owner, bookings OfferingBookings) *BookingProjection {
	return &BookingProjection{Owner: owner, bookings: bookings}
}

// AllCareOfferingLinks lists every booking of the tenant's request children.
func (p *BookingProjection) AllCareOfferingLinks(ctx context.Context) ([]CareOfferingLink, error) {
	return p.CareExitOfferingLinks(ctx, nil)
}

// CareExitOfferingLinks lists the bookings of the children whose application
// created or matched one of the students.
func (p *BookingProjection) CareExitOfferingLinks(ctx context.Context, studentIDs []int64) ([]CareOfferingLink, error) {
	children, err := p.CareExitApplicationLinks(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.ID)
	}
	return p.offeringLinks(ctx, ids)
}

// ApprovedBookingOfferingLinks lists the bookings of approved children.
func (p *BookingProjection) ApprovedBookingOfferingLinks(ctx context.Context) ([]CareOfferingLink, error) {
	children, err := p.CareExitApplicationLinks(ctx, nil)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		if child.Status == ChildStatusApproved {
			ids = append(ids, child.ID)
		}
	}
	return p.offeringLinks(ctx, ids)
}

func (p *BookingProjection) offeringLinks(ctx context.Context, ids []int64) ([]CareOfferingLink, error) {
	bookings, err := p.bookings.CareOfferingBookingHistory(ctx, ids)
	if err != nil {
		return nil, err
	}
	links := make([]CareOfferingLink, 0, len(bookings))
	for _, booking := range bookings {
		links = append(links, CareOfferingLink{ID: booking.ID, TenantID: booking.TenantID,
			RequestChildID: booking.RequestChildID, CareOfferingID: booking.CareOfferingID, SelectedDays: booking.SelectedDays,
			ValidFrom: copyDate(booking.ValidFrom), ValidUntil: copyDate(booking.ValidUntil)})
	}
	sort.Slice(links, func(i, j int) bool { return links[i].ID < links[j].ID })
	return links, nil
}

func effectiveSelections(bookings []OfferingBooking) []*RequestChildOffering {
	rows := make([]*RequestChildOffering, 0, len(bookings))
	for _, booking := range bookings {
		rows = append(rows, &RequestChildOffering{
			ID: booking.ID, TenantID: booking.TenantID, RequestChildID: booking.RequestChildID, CareOfferingID: booking.CareOfferingID,
			SelectedDays: booking.SelectedDays, ManualSelectedDays: booking.ManualSelectedDays, AutomaticSelectedDays: booking.AutomaticSelectedDays,
			CreatedAt: booking.CreatedAt, UpdatedAt: booking.UpdatedAt,
			ValidFrom: copyDate(booking.ValidFrom), ValidUntil: copyDate(booking.ValidUntil),
		})
	}
	return rows
}

func copyDate(date *Date) *Date {
	if date == nil {
		return nil
	}
	return new(*date)
}
