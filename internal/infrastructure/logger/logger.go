package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger provides structured logging functionality
type Logger struct {
	zap *zap.Logger
}

// NewLogger creates a new logger instance
func NewLogger(environment string) *Logger {
	var config zap.Config

	if environment == "production" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	// Ensure output goes to stdout
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	config.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	logger, err := config.Build()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}

	return &Logger{zap: logger}
}

// WithContext creates a logger with context information
func (l *Logger) WithContext(ctx context.Context) *Logger {
	var fields []zap.Field

	if requestID, ok := ctx.Value("request_id").(string); ok && requestID != "" {
		fields = append(fields, zap.String("X-TRACE-ID", requestID))
	}

	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		fields = append(fields, zap.String("user_id", userID))
	}

	return &Logger{zap: l.zap.With(fields...)}
}

// WithRequest creates a logger with HTTP request information
func (l *Logger) WithRequest(ctx context.Context, method, path, clientIP string, statusCode int, latency string, dataLength int) *Logger {
	fields := []zap.Field{
		zap.String("logger", "Middleware"),
		zap.String("msg", "REST Processed"),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("clientIP", clientIP),
		zap.Int("statusCode", statusCode),
		zap.String("latency", latency),
		zap.Int("dataLength", dataLength),
	}

	if requestID, ok := ctx.Value("request_id").(string); ok && requestID != "" {
		fields = append(fields, zap.String("X-TRACE-ID", requestID))
	}

	return &Logger{zap: l.zap.With(fields...)}
}

// Info logs an info level message
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.zap.Info(msg, fields...)
}

// Error logs an error level message
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.zap.Error(msg, fields...)
}

// Warn logs a warning level message
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.zap.Warn(msg, fields...)
}

// Debug logs a debug level message
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.zap.Debug(msg, fields...)
}

// Fatal logs a fatal level message and exits
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.zap.Fatal(msg, fields...)
	os.Exit(1)
}

// logWithCustomFormat logs with the custom [LEVEL] : {} format
func (l *Logger) logWithCustomFormat(level, msg string, fields ...zap.Field) {
	data := make(map[string]interface{})
	data["msg"] = msg

	for _, field := range fields {
		if field.Interface != nil {
			switch field.Key {
			case "X-TRACE-ID", "user_id", "method", "path", "statusCode", "latency", "error":
				data[field.Key] = field.Interface
			}
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("[%s] : %s\n", level, msg)
		return
	}

	fmt.Printf("[%s] : %s\n", level, string(jsonData))
}

// WithError adds error information to the logger
func (l *Logger) WithError(err error) *Logger {
	return &Logger{zap: l.zap.With(zap.Error(err))}
}

// WithField adds a single field to the logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{zap: l.zap.With(zap.Any(key, value))}
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for key, value := range fields {
		zapFields = append(zapFields, zap.Any(key, value))
	}
	return &Logger{zap: l.zap.With(zapFields...)}
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.zap.Sync()
}
