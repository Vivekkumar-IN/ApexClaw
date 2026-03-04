package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	levelDebug = 0
	levelInfo  = 1
	levelWarn  = 2
	levelError = 3
)

var Log *Logger

type Logger struct {
	level     int
	filePath  string
	file      *os.File
	mu        sync.Mutex
	maxBytes  int64
	fileSize  int64
}

func InitLogger() {
	levelStr := strings.ToUpper(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
	level := levelInfo
	switch levelStr {
	case "DEBUG":
		level = levelDebug
	case "WARN":
		level = levelWarn
	case "ERROR":
		level = levelError
	case "INFO":
		level = levelInfo
	}

	logDir := filepath.Join(os.ExpandEnv("$HOME"), ".apexclaw", "logs")
	os.MkdirAll(logDir, 0755)

	logFile := filepath.Join(logDir, "app.log")

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		Log = &Logger{level: level, filePath: logFile, file: nil, maxBytes: 10 * 1024 * 1024}
		return
	}

	stat, _ := f.Stat()
	fileSize := int64(0)
	if stat != nil {
		fileSize = stat.Size()
	}

	Log = &Logger{
		level:    level,
		filePath: logFile,
		file:     f,
		maxBytes: 10 * 1024 * 1024,
		fileSize: fileSize,
	}
}

func (l *Logger) Debugf(format string, args ...any) {
	l.log(levelDebug, "DEBUG", format, args...)
}

func (l *Logger) Infof(format string, args ...any) {
	l.log(levelInfo, "INFO", format, args...)
}

func (l *Logger) Warnf(format string, args ...any) {
	l.log(levelWarn, "WARN", format, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.log(levelError, "ERROR", format, args...)
}

func (l *Logger) log(level int, label, format string, args ...any) {
	if l == nil || level < l.level {
		return
	}

	msg := fmt.Sprintf("[%s] %s %s\n", label, time.Now().Format("2006/01/02 15:04:05"), fmt.Sprintf(format, args...))

	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Print(msg)

	if l.file != nil {
		l.file.WriteString(msg)
		l.fileSize += int64(len(msg))
		l.maybeRotate()
	}
}

func (l *Logger) maybeRotate() {
	if l == nil || l.file == nil || l.fileSize < l.maxBytes {
		return
	}

	l.file.Close()

	// Rotate: app.log.2 → delete, app.log.1 → app.log.2, app.log → app.log.1
	os.Remove(l.filePath + ".2")
	os.Rename(l.filePath+".1", l.filePath+".2")
	os.Rename(l.filePath, l.filePath+".1")

	// Open new file
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to rotate log file: %v\n", err)
		l.file = nil
		return
	}

	l.file = f
	l.fileSize = 0
}

func (l *Logger) Close() {
	if l == nil || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.file.Close()
}
