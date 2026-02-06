package geminicli

// Logger represents the interface for logging operations
type Logger interface {
	DebugWith(msg string, keysAndValues ...interface{})
	InfoWith(msg string, keysAndValues ...interface{})
	WarnWith(msg string, keysAndValues ...interface{})
	ErrorWith(msg string, keysAndValues ...interface{})
}

// NoOpLogger is a logger that discards all log messages
type NoOpLogger struct{}

func (l NoOpLogger) DebugWith(msg string, keysAndValues ...interface{}) {}
func (l NoOpLogger) InfoWith(msg string, keysAndValues ...interface{})  {}
func (l NoOpLogger) WarnWith(msg string, keysAndValues ...interface{})  {}
func (l NoOpLogger) ErrorWith(msg string, keysAndValues ...interface{}) {}

// NewNoOpLogger creates a new NoOpLogger
func NewNoOpLogger() Logger {
	return &NoOpLogger{}
}
