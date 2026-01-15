// Package logger provides structured logging for core services.
// It avoids adapter-level dependencies by exposing a minimal logger interface.
package logger

// StructuredLogger defines the logging methods used inside the core layer.
// It supports basic structured fields and chaining.
type StructuredLogger interface {
	WithField(key string, value any) StructuredLogger
	WithFields(fields map[string]any) StructuredLogger
	WithError(err error) StructuredLogger
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
}

type noopLogger struct{}

func (noopLogger) WithField(string, any) StructuredLogger     { return noopLogger{} }
func (noopLogger) WithFields(map[string]any) StructuredLogger { return noopLogger{} }
func (noopLogger) WithError(error) StructuredLogger           { return noopLogger{} }
func (noopLogger) Debug(...any)                               {}
func (noopLogger) Info(...any)                                {}
func (noopLogger) Warn(...any)                                {}
func (noopLogger) Error(...any)                               {}

// Logger is the core logger instance. It defaults to a no-op implementation.
var Logger StructuredLogger = noopLogger{}

// SetLogger configures the core logger implementation.
// Passing nil resets the logger to a no-op implementation.
func SetLogger(l StructuredLogger) {
	if l == nil {
		Logger = noopLogger{}
		return
	}
	Logger = l
}
