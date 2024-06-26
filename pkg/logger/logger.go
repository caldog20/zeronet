package logger

import (
	"fmt"

	"github.com/fatih/color"
)

const (
	LogBuffer = 10

	// LogLevel
	LogLevelNone = iota
	LogLevelInfo = iota
	LogLevelWarn
	LogLevelError
	LogLevelDebug
)

type Logger struct {
	logs chan string

	// Prefix for logs
	Prefix   string
	LogLevel int
}

func New() *Logger {
	return &Logger{
		logs: make(chan string, LogBuffer),
	}
}

func (l *Logger) SetLogLevel(level int) {
	if level < LogLevelNone {
		level = LogLevelNone
	} else if level > LogLevelDebug {
		level = LogLevelDebug
	} else {
		l.LogLevel = level
	}
}

func (l *Logger) SetPrefix(prefix string) {
	l.Prefix = prefix
}

func (l *Logger) Logs() <-chan string {
	return l.logs
}

func (l *Logger) Close() {
	close(l.logs)
}

func (l *Logger) Printf(format string, v ...any) {
	l.logs <- fmt.Sprintf(format, v...)
}

func (l *Logger) Println(s ...any) {
	l.logs <- fmt.Sprintln(s...)
}

func (l *Logger) Print(v ...any) {
	l.logs <- fmt.Sprint(v...)
}

func (l *Logger) Info(v ...any) {
	if l.LogLevel >= LogLevelInfo {
		l.Print(fmt.Sprintf("INFO: %s", v...))
	}

}

func (l *Logger) Infof(format string, v ...any) {
	if l.LogLevel >= LogLevelInfo {
		l.Printf("INFO: "+format, v...)
	}
}

func (l *Logger) Warn(v ...any) {
	if l.LogLevel >= LogLevelWarn {
		c := color.New(color.FgYellow)
		l.Print(c.Sprintf("WARN: %s", v...))
	}
}

func (l *Logger) Warnf(format string, v ...any) {
	if l.LogLevel >= LogLevelWarn {
		c := color.New(color.FgYellow)
		l.Printf(c.Sprintf("WARN: "+format, v...))
	}
}

func (l *Logger) Error(v ...any) {
	if l.LogLevel >= LogLevelError {
		c := color.New(color.FgRed)
		l.Print(c.Sprintf("ERROR: %s", v...))
	}
}

func (l *Logger) Errorf(format string, v ...any) {
	if l.LogLevel >= LogLevelError {
		c := color.New(color.FgRed)
		l.Printf(c.Sprintf("ERROR: "+format, v...))
	}
}

func (l *Logger) Debug(v ...any) {
	if l.LogLevel >= LogLevelDebug {
		c := color.New(color.FgGreen)
		l.Print(c.Sprintf("DEBUG: %s", v...))
	}
}

func (l *Logger) Debugf(format string, v ...any) {
	if l.LogLevel >= LogLevelDebug {
		c := color.New(color.FgGreen)
		l.Printf(c.Sprintf("ERROR: "+format, v...))
	}
}
