package sylph

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	t.Run("should create logger with default config", func(t *testing.T) {
		logger := DefaultLogger("test")
		assert.NotNil(t, logger)
		assert.NotNil(t, logger.entry)
		assert.Equal(t, defaultLoggerConfig, logger.opt)
	})

	t.Run("should create logger with custom config", func(t *testing.T) {
		config := &LoggerConfig{
			Async: true,
		}
		logger := NewLogger("test", config)
		assert.NotNil(t, logger)
		assert.NotNil(t, logger.entry)
		assert.Equal(t, config, logger.opt)
	})
}

func TestLoggerMethods(t *testing.T) {
	logger := DefaultLogger("test")
	message := &LoggerMessage{
		Message: "test message",
	}

	tests := []struct {
		name   string
		method func(*LoggerMessage)
		level  logrus.Level
	}{
		{"Info", logger.Info, logrus.InfoLevel},
		{"Trace", logger.Trace, logrus.TraceLevel},
		{"Debug", logger.Debug, logrus.DebugLevel},
		{"Warn", logger.Warn, logrus.WarnLevel},
		{"Fatal", logger.Fatal, logrus.FatalLevel},
		{"Panic", logger.Panic, logrus.PanicLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.method(message)
			// TODO: Add assertions to verify log output
		})
	}
}

func TestLoggerError(t *testing.T) {
	logger := DefaultLogger("test")
	message := &LoggerMessage{
		Message: "test error",
	}
	err := assert.AnError

	logger.Error(message, err)
	assert.Equal(t, err.Error(), message.Error)
	// TODO: Add assertions to verify error log output
}

func TestLoggerAsync(t *testing.T) {
	config := &LoggerConfig{
		Async: true,
	}
	logger := NewLogger("test", config)
	message := &LoggerMessage{
		Message: "async test",
	}

	logger.Info(message)
	// TODO: Add assertions to verify async behavior
}

func TestLoggerRecover(t *testing.T) {
	// TODO: Add test case for recover mechanism
}
