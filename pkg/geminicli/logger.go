package geminicli

// Logger represents the interface for logging operations
type Logger interface {
	DebugWith(msg string, keysAndValues ...any)
	InfoWith(msg string, keysAndValues ...any)
	WarnWith(msg string, keysAndValues ...any)
	ErrorWith(msg string, keysAndValues ...any)
}

// NoOpLogger is a logger that discards all log messages
type NoOpLogger struct{}

func (l NoOpLogger) DebugWith(msg string, keysAndValues ...any) {}
func (l NoOpLogger) InfoWith(msg string, keysAndValues ...any)  {}
func (l NoOpLogger) WarnWith(msg string, keysAndValues ...any)  {}
func (l NoOpLogger) ErrorWith(msg string, keysAndValues ...any) {}

// NewNoOpLogger creates a new NoOpLogger
func NewNoOpLogger() Logger {
	return &NoOpLogger{}
}
