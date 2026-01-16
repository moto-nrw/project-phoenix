package logger

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Logger Initialization Tests
// =============================================================================

func TestLogger_InitializedOnImport(t *testing.T) {
	// Logger should be initialized by init()
	assert.NotNil(t, Logger)
}

func TestLogger_IsLogrusLogger(t *testing.T) {
	// Logger should be a logrus.Logger
	assert.IsType(t, &logrus.Logger{}, Logger)
}

func TestLogger_CanLog(t *testing.T) {
	// Logger should be functional
	assert.NotPanics(t, func() {
		Logger.Debug("test debug message")
		Logger.Info("test info message")
		Logger.Warn("test warning message")
	})
}

func TestLogger_SupportsFields(t *testing.T) {
	// Logger should support structured logging with fields
	assert.NotPanics(t, func() {
		Logger.WithFields(logrus.Fields{
			"user_id": 123,
			"action":  "test",
		}).Info("test with fields")
	})
}

func TestLogger_SupportsWithField(t *testing.T) {
	// Logger should support single field logging
	assert.NotPanics(t, func() {
		Logger.WithField("key", "value").Info("test with single field")
	})
}

// =============================================================================
// Logger Configuration Tests
// =============================================================================

func TestLogger_DefaultLevel(t *testing.T) {
	// Default logger should have a level set
	assert.NotEqual(t, logrus.PanicLevel, Logger.Level)
}

func TestLogger_CanSetLevel(t *testing.T) {
	// Should be able to change log level
	originalLevel := Logger.Level

	Logger.SetLevel(logrus.DebugLevel)
	assert.Equal(t, logrus.DebugLevel, Logger.Level)

	// Restore original level
	Logger.SetLevel(originalLevel)
}

func TestLogger_CanSetFormatter(t *testing.T) {
	// Should be able to set a formatter
	originalFormatter := Logger.Formatter

	Logger.SetFormatter(&logrus.JSONFormatter{})
	assert.IsType(t, &logrus.JSONFormatter{}, Logger.Formatter)

	// Restore original formatter
	Logger.SetFormatter(originalFormatter)
}

// =============================================================================
// Logger Entry Tests
// =============================================================================

func TestLogger_WithError(t *testing.T) {
	// Logger should support error field
	assert.NotPanics(t, func() {
		err := assert.AnError
		Logger.WithError(err).Error("test error logging")
	})
}

func TestLogger_ChainedFields(t *testing.T) {
	// Logger should support chained field calls
	assert.NotPanics(t, func() {
		Logger.
			WithField("field1", "value1").
			WithField("field2", "value2").
			WithFields(logrus.Fields{"field3": "value3"}).
			Info("chained fields test")
	})
}
