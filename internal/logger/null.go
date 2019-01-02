package logger

type nullLogger struct {
}

func (l *nullLogger) Verbose(message string) {
}

func (l *nullLogger) Verbosef(format string, a ...interface{}) {
}

func (l *nullLogger) Info(message string) {
}

func (l *nullLogger) Infof(format string, a ...interface{}) {
}

func (l *nullLogger) Warning(message string) {
}

func (l *nullLogger) Warningf(format string, a ...interface{}) {
}

func (l *nullLogger) Error(message string) error {
	return nil
}

func (l *nullLogger) Errorf(format string, a ...interface{}) error {
	return nil
}

// NewNull logger
func NewNull() Logger {
	return &nullLogger{}
}
