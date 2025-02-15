package logging

import (
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogMessage represents a single log entry
type LogMessage struct {
	Level   zapcore.Level
	Message string
	Time    time.Time
}

// ChannelCore is a custom zapcore.Core that writes logs to a channel
type ChannelCore struct {
	LevelEnabler zapcore.LevelEnabler
	output       chan LogMessage
}

// Enabled checks if the log level is enabled
func (c *ChannelCore) Enabled(level zapcore.Level) bool {
	return c.LevelEnabler.Enabled(level)
}

// With adds structured context to the Core
func (c *ChannelCore) With(fields []zapcore.Field) zapcore.Core {
	return c // No structured context is used here
}

// Check determines whether the supplied Entry should be logged
func (c *ChannelCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

// Write writes the log entry to the channel
func (c *ChannelCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	logMsg := LogMessage{
		Level:   entry.Level,
		Message: entry.Message,
		Time:    entry.Time,
	}
	select {
	case c.output <- logMsg:
	default: // Channel full; drop log or handle as needed
	}
	return nil
}

// Sync flushes buffered logs (no-op in this case)
func (c *ChannelCore) Sync() error {
	return nil
}

// DynamicWriteSyncer manages multiple write syncers
type DynamicWriteSyncer struct {
	mu      sync.Mutex
	writers []zapcore.WriteSyncer
}

func (d *DynamicWriteSyncer) AddWriteSyncer(ws zapcore.WriteSyncer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writers = append(d.writers, ws)
}

func (d *DynamicWriteSyncer) Write(p []byte) (n int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, ws := range d.writers {
		n, err = ws.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

func (d *DynamicWriteSyncer) Sync() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, ws := range d.writers {
		if err := ws.Sync(); err != nil {
			return err
		}
	}
	return nil
}

// loggerSingleton holds the singleton instance of the logger
type loggerSingleton struct {
	logger        *zap.SugaredLogger
	logsCh        chan LogMessage
	atomicLevel   zap.AtomicLevel
	dynamicSyncer *DynamicWriteSyncer
}

var (
	instance *loggerSingleton
	once     sync.Once
)

func (ls *loggerSingleton) GetLogCh() chan LogMessage {
	return ls.logsCh
}

// GetLogger returns the singleton SugaredLogger instance
func GetLogger() *zap.SugaredLogger {
	return GetInstance().logger
}

// GetInstance initializes the singleton instance if it doesn't exist
func GetInstance() *loggerSingleton {
	once.Do(func() {
		instance = &loggerSingleton{}

		// Default initialization with INFO level
		initLogger(instance, zapcore.InfoLevel, defaultEncoderConfig())
	})
	return instance
}

// initLogger initializes the logger with the given level and encoder config
func initLogger(ls *loggerSingleton, level zapcore.Level, encoderConfig zapcore.EncoderConfig) {
	logChannel := make(chan LogMessage, 100) // Buffer size of 100
	ls.logsCh = logChannel

	// Create ChannelCore
	channelCore := &ChannelCore{
		LevelEnabler: level,
		output:       logChannel,
	}

	// Create DynamicWriteSyncer
	ls.dynamicSyncer = &DynamicWriteSyncer{}
	multiCore := zapcore.NewTee(
		channelCore, // Logs to the channel
		zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), zapcore.AddSync(os.Stdout), level),
	)

	// Build the logger
	logger := zap.New(multiCore, zap.AddCaller())
	ls.logger = logger.Sugar()
	ls.atomicLevel = zap.NewAtomicLevelAt(level)
}

// defaultEncoderConfig returns a simple encoder configuration
func defaultEncoderConfig() zapcore.EncoderConfig {
	// Custom time encoder for yymmdd-hh:mm format
	customTimeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("02-01-2006 15:04")) // yymmdd-hh:mm format
	}
	return zapcore.EncoderConfig{
		TimeKey:  "ts",
		LevelKey: "level",
		NameKey:  "logger",
		//CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "",
		EncodeTime:    customTimeEncoder,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}
}

// AddFileOutput adds a file write syncer to the logger
func AddFileOutput(logFilePath string) error {
	ls := GetInstance()
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	fileSyncer := zapcore.AddSync(logFile)
	ls.dynamicSyncer.AddWriteSyncer(fileSyncer)
	ls.logger.Infow("Added file output", "file", logFilePath)
	return nil
}

// ChangeLevel dynamically changes the logging level
func ChangeLevel(newLevel zapcore.Level) {
	ls := GetInstance()
	ls.atomicLevel.SetLevel(newLevel)
	ls.logger.Infow("Log level changed", "new_level", newLevel.String())
}

// GetLogsChannel returns the logs channel
func GetLogsChannel() <-chan LogMessage {
	return GetInstance().logsCh
}
