package active

import "github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"

// EventPublisher is the realtime fan-out the retained presence and workforce
// services announce changes through. It is the Delivery owner's publisher
// contract; the composition root supplies the realtime hub.
type EventPublisher interface {
	realtimeevents.Publisher
}
