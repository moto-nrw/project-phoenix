package sse

// Event exposes the SSE name while its native value keeps the existing JSON
// representation. Root selects the Delivery event type.
type Event interface{ SSEType() string }

// Client exposes the native event channel. The port adds no queue or relay.
type Client[E Event] interface{ SSEEvents() <-chan E }

// Hub registers the native client type selected by Root.
type Hub[C any] interface {
	Register(C, int64, []string)
	RegisterParent(C)
	RegisterSchool(C, int64)
	Unregister(C)
}
