package checkin

import (
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// checkinResult holds the result of processing a checkin request
type checkinResult struct {
	Action           string
	VisitID          *int64
	RoomName         string
	PreviousRoomName string
	GreetingMsg      string
}

// checkinResultInput holds the input parameters for building a checkin result.
// This struct reduces the parameter count of buildCheckinResult for better maintainability.
type checkinResultInput struct {
	Student          *users.Student
	Person           *users.Person
	CheckedOut       bool
	NewVisitID       *int64
	CheckoutVisitID  *int64
	RoomName         string
	PreviousRoomName string
	CurrentVisit     *active.Visit
}

// checkinProcessingInput holds the inputs for processing a student checkin
type checkinProcessingInput struct {
	RoomID       *int64
	SkipCheckin  bool
	CheckedOut   bool
	CurrentVisit *active.Visit
}

// checkinProcessingResult holds the result of checkin processing
type checkinProcessingResult struct {
	NewVisitID *int64
	RoomName   string
	Error      error
}
