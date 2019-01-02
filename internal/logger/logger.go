package logger

import (
	"errors"
	"fmt"
	"time"

	"github.com/elijahglover/inbound/internal/config"
)

type logEntry struct {
	time     time.Time
	severity string
	message  string
}

type channelLogger struct {
	config  *config.Config
	channel chan logEntry
}

// Verbose passes log to channel
func (l *channelLogger) Verbose(message string) {
	if l.config.LogVerbose {
		l.channel <- logEntry{time: time.Now(), severity: LogVerbose, message: message}
	}
}

// Verbosef formatted passes log to channel
func (l *channelLogger) Verbosef(format string, a ...interface{}) {
	if l.config.LogVerbose {
		l.Verbose(fmt.Sprintf(format, a...))
	}
}

// Info passes log to channel
func (l *channelLogger) Info(message string) {
	if l.config.LogInfo {
		l.channel <- logEntry{time: time.Now(), severity: LogInfo, message: message}
	}
}

// Infof formatted passes log to channel
func (l *channelLogger) Infof(format string, a ...interface{}) {
	if l.config.LogInfo {
		l.Info(fmt.Sprintf(format, a...))
	}
}

// Warning passes log to channel
func (l *channelLogger) Warning(message string) {
	if l.config.LogWarning {
		l.channel <- logEntry{time: time.Now(), severity: LogWarning, message: message}
	}
}

// Warningf formatted passes log to channel
func (l *channelLogger) Warningf(format string, a ...interface{}) {
	if l.config.LogWarning {
		l.Warning(fmt.Sprintf(format, a...))
	}
}

// Error passes log to channel
func (l *channelLogger) Error(message string) error {
	l.channel <- logEntry{time: time.Now(), severity: LogError, message: message}
	return errors.New(message)
}

// Errorf formatted passes log to channel
func (l *channelLogger) Errorf(format string, a ...interface{}) error {
	return l.Error(fmt.Sprintf(format, a...))
}

func (l *channelLogger) Output() error {
	for {
		entry := <-l.channel
		fmt.Printf("%s - [%v] %v\n", entry.time.Format(time.RFC3339), entry.severity, entry.message)
	}
}

// NewStdOut logger
func NewStdOut(config *config.Config) Logger {
	loggerStdOut := &channelLogger{
		config:  config,
		channel: make(chan logEntry),
	}

	go loggerStdOut.Output()
	return loggerStdOut
}
