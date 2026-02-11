package geminicli

// LoggerAdapter adapts the main package logger to the geminicli Logger interface
type LoggerAdapter struct {
	debugWith func(msg string, keysAndValues ...any)
	infoWith  func(msg string, keysAndValues ...any)
	warnWith  func(msg string, keysAndValues ...any)
	errorWith func(msg string, keysAndValues ...any)
}

// NewLoggerAdapter creates a new logger adapter with the provided logging functions
func NewLoggerAdapter(
	debugWith func(msg string, keysAndValues ...any),
	infoWith func(msg string, keysAndValues ...any),
	warnWith func(msg string, keysAndValues ...any),
	errorWith func(msg string, keysAndValues ...any),
) Logger {
	return &LoggerAdapter{
		debugWith: debugWith,
		infoWith:  infoWith,
		warnWith:  warnWith,
		errorWith: errorWith,
	}
}

func (a *LoggerAdapter) DebugWith(msg string, keysAndValues ...any) {
	if a.debugWith != nil {
		a.debugWith(msg, keysAndValues...)
	}
}

func (a *LoggerAdapter) InfoWith(msg string, keysAndValues ...any) {
	if a.infoWith != nil {
		a.infoWith(msg, keysAndValues...)
	}
}

func (a *LoggerAdapter) WarnWith(msg string, keysAndValues ...any) {
	if a.warnWith != nil {
		a.warnWith(msg, keysAndValues...)
	}
}

func (a *LoggerAdapter) ErrorWith(msg string, keysAndValues ...any) {
	if a.errorWith != nil {
		a.errorWith(msg, keysAndValues...)
	}
}
