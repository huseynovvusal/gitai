package geminicli

// LoggerAdapter adapts the main package logger to the geminicli Logger interface
type LoggerAdapter struct {
	debugWith func(msg string, keysAndValues ...interface{})
	infoWith  func(msg string, keysAndValues ...interface{})
	warnWith  func(msg string, keysAndValues ...interface{})
	errorWith func(msg string, keysAndValues ...interface{})
}

// NewLoggerAdapter creates a new logger adapter with the provided logging functions
func NewLoggerAdapter(
	debugWith func(msg string, keysAndValues ...interface{}),
	infoWith func(msg string, keysAndValues ...interface{}),
	warnWith func(msg string, keysAndValues ...interface{}),
	errorWith func(msg string, keysAndValues ...interface{}),
) Logger {
	return &LoggerAdapter{
		debugWith: debugWith,
		infoWith:  infoWith,
		warnWith:  warnWith,
		errorWith: errorWith,
	}
}

func (a *LoggerAdapter) DebugWith(msg string, keysAndValues ...interface{}) {
	if a.debugWith != nil {
		a.debugWith(msg, keysAndValues...)
	}
}

func (a *LoggerAdapter) InfoWith(msg string, keysAndValues ...interface{}) {
	if a.infoWith != nil {
		a.infoWith(msg, keysAndValues...)
	}
}

func (a *LoggerAdapter) WarnWith(msg string, keysAndValues ...interface{}) {
	if a.warnWith != nil {
		a.warnWith(msg, keysAndValues...)
	}
}

func (a *LoggerAdapter) ErrorWith(msg string, keysAndValues ...interface{}) {
	if a.errorWith != nil {
		a.errorWith(msg, keysAndValues...)
	}
}
