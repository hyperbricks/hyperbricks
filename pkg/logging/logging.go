package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogMessage keeps the existing dashboard contract and adds structured context.
type LogMessage struct {
	Level   zapcore.Level
	Message string
	Time    time.Time
	Logger  string
	Fields  map[string]interface{} `json:",omitempty"`
}

type ChannelCore struct {
	LevelEnabler zapcore.LevelEnabler
	output       chan LogMessage
	owner        *loggerSingleton
	fields       []zapcore.Field
}

func (c *ChannelCore) Enabled(level zapcore.Level) bool { return c.LevelEnabler.Enabled(level) }

func (c *ChannelCore) With(fields []zapcore.Field) zapcore.Core {
	clone := *c
	clone.fields = append(append([]zapcore.Field(nil), c.fields...), fields...)
	return &clone
}

func (c *ChannelCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func (c *ChannelCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	encoder := zapcore.NewMapObjectEncoder()
	for _, field := range c.fields {
		field.AddTo(encoder)
	}
	for _, field := range fields {
		field.AddTo(encoder)
	}
	message := LogMessage{Level: entry.Level, Message: entry.Message, Time: entry.Time,
		Logger: entry.LoggerName, Fields: encoder.Fields}
	ls := c.owner
	if ls == nil {
		ls = GetInstance()
	}
	ls.mu.Lock()
	if len(ls.logBuffer) >= 10 {
		copy(ls.logBuffer, ls.logBuffer[1:])
		ls.logBuffer = ls.logBuffer[:9]
	}
	ls.logBuffer = append(ls.logBuffer, message)
	ls.mu.Unlock()
	message.Fields = cloneFields(message.Fields)
	select {
	case c.output <- message:
	default:
	}
	return nil
}

func (c *ChannelCore) Sync() error { return nil }

// DynamicWriteSyncer allows file outputs to be attached without replacing loggers.
type DynamicWriteSyncer struct {
	mu      sync.Mutex
	writers []zapcore.WriteSyncer
	files   map[string]*os.File
}

func (d *DynamicWriteSyncer) AddWriteSyncer(writer zapcore.WriteSyncer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writers = append(d.writers, writer)
}

func (d *DynamicWriteSyncer) Write(data []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var errs []error
	for _, writer := range d.writers {
		n, err := writer.Write(data)
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
		errs = append(errs, err)
	}
	return len(data), errors.Join(errs...)
}

func (d *DynamicWriteSyncer) Sync() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var errs []error
	for _, writer := range d.writers {
		errs = append(errs, writer.Sync())
	}
	return errors.Join(errs...)
}

func (d *DynamicWriteSyncer) addFile(name string) error {
	name, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.files[name] != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if d.files == nil {
		d.files = make(map[string]*os.File)
	}
	d.files[name] = file
	d.writers = append(d.writers, zapcore.AddSync(file))
	return nil
}

func (d *DynamicWriteSyncer) close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var errs []error
	for _, file := range d.files {
		errs = append(errs, file.Sync(), file.Close())
	}
	d.files = nil
	d.writers = nil
	return errors.Join(errs...)
}

type loggerSingleton struct {
	logger        *zap.SugaredLogger
	logsCh        chan LogMessage
	atomicLevel   zap.AtomicLevel
	dynamicSyncer *DynamicWriteSyncer
	logBuffer     []LogMessage
	mu            sync.Mutex
	configMu      sync.RWMutex
	terminal      zapcore.WriteSyncer
	color         bool
}

var (
	instance *loggerSingleton
	once     sync.Once
)

func GetInstance() *loggerSingleton {
	once.Do(func() { instance = newLogger(os.Stderr, ColorEnabled(os.Stderr)) })
	return instance
}

func newLogger(output io.Writer, color bool) *loggerSingleton {
	ls := &loggerSingleton{logsCh: make(chan LogMessage, 100), atomicLevel: zap.NewAtomicLevelAt(zap.InfoLevel),
		dynamicSyncer: &DynamicWriteSyncer{}, terminal: zapcore.Lock(zapcore.AddSync(output)), color: color}
	_ = ls.configure("info", "console")
	return ls
}

func GetLogger() *zap.SugaredLogger {
	ls := GetInstance()
	ls.configMu.RLock()
	defer ls.configMu.RUnlock()
	return ls.logger
}

// VerboseEnabled reports whether DEBUG events are currently visible.
func VerboseEnabled() bool {
	return GetLogger().Desugar().Core().Enabled(zapcore.DebugLevel)
}

func (ls *loggerSingleton) GetLogCh() chan LogMessage { return ls.logsCh }
func GetLogsChannel() <-chan LogMessage               { return GetInstance().logsCh }

func GetLogs() []LogMessage {
	ls := GetInstance()
	ls.mu.Lock()
	defer ls.mu.Unlock()
	result := append([]LogMessage{}, ls.logBuffer...)
	for index := range result {
		result[index].Fields = cloneFields(result[index].Fields)
	}
	return result
}

// Configure is called before serving. Existing loggers share the dynamic level.
func Configure(level, format string) error { return GetInstance().configure(level, format) }

func (ls *loggerSingleton) configure(level, format string) error {
	parsed, err := zapcore.ParseLevel(strings.ToLower(level))
	if err != nil {
		return fmt.Errorf("invalid log level %q: use debug, info, warn, error, dpanic, panic or fatal", level)
	}
	if format != "console" && format != "json" {
		return fmt.Errorf("invalid log format %q: use console or json", format)
	}
	config := zap.NewProductionEncoderConfig()
	config.TimeKey, config.MessageKey = "time", "message"
	config.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	var terminal zapcore.Core = zapcore.NewCore(zapcore.NewJSONEncoder(config), ls.terminal, ls.atomicLevel)
	if format == "console" {
		terminal = &consoleCore{LevelEnabler: ls.atomicLevel, output: ls.terminal, color: ls.color}
	}
	fileConfig := zap.NewProductionEncoderConfig()
	fileConfig.TimeKey, fileConfig.MessageKey = "time", "message"
	fileConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	channel := &ChannelCore{LevelEnabler: ls.atomicLevel, output: ls.logsCh, owner: ls}
	core := zapcore.NewTee(channel, terminal,
		zapcore.NewCore(zapcore.NewJSONEncoder(fileConfig), ls.dynamicSyncer, ls.atomicLevel))
	logger := zap.New(&safeCore{Core: core}, zap.ErrorOutput(ls.terminal),
		zap.WithFatalHook(fatalHook{files: ls.dynamicSyncer}))
	ls.configMu.Lock()
	ls.logger = logger.Sugar()
	ls.atomicLevel.SetLevel(parsed)
	ls.configMu.Unlock()
	return nil
}

type fatalHook struct{ files *DynamicWriteSyncer }

func (hook fatalHook) OnWrite(*zapcore.CheckedEntry, []zapcore.Field) {
	_ = hook.files.close()
	os.Exit(1)
}

func AddFileOutput(path string) error { return GetInstance().dynamicSyncer.addFile(path) }
func ChangeLevel(level zapcore.Level) { GetInstance().atomicLevel.SetLevel(level) }
func Close() error                    { return GetInstance().dynamicSyncer.close() }
