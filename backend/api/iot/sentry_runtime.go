package iot

// SentryTarget is the two scalar routing facts returned by the public parser.
type SentryTarget struct {
	ProjectID   string
	EnvelopeURL string
}

// SentryTransportRuntime binds project validation and tagging without moving relay I/O into Root.
type SentryTransportRuntime struct {
	ParseTarget func(string) (SentryTarget, error)
	TagEnvelope func([]byte, string, ErrorReportOrigin) ([]byte, error)
}
