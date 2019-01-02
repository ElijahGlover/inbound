package logger

const (
	// LogVerbose flag
	LogVerbose = "VERBOSE"
	// LogInfo flag
	LogInfo = "INFO"
	// LogWarning flag
	LogWarning = "WARNING"
	// LogError flag
	LogError = "ERROR"
)

// Logger represents a standard logger interface
type Logger interface {
	Verbose(message string)
	Verbosef(format string, a ...interface{})
	Info(message string)
	Infof(format string, a ...interface{})
	Warning(message string)
	Warningf(format string, a ...interface{})
	Error(message string) error
	Errorf(format string, a ...interface{}) error
}
