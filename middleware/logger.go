// Copyright (c) 2025 Neomantra Corp

package middleware

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// SetGinLogger is a functor to sets the logger on a gin.Context.
func SetGinLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("gin-logger", logger)
		c.Next()
	}
}

// GetGinLogger returns the current logger from a gin.Context
func GetGinLogger(c *gin.Context) *zap.Logger {
	l, ok := c.Get("gin-logger")
	if !ok {
		return zap.NewNop()
	}
	return l.(*zap.Logger)
}

// CreateLogger creates a logger for our app
// Honors APP_LOG_PATH environment variable, otherwise uses current directory
func CreateLogger(appName string, isRelease bool) *zap.Logger {
	logPath := os.Getenv("APP_LOG_PATH")
	if logPath == "" {
		logPath = "."
	}

	// log rotation
	file := zapcore.AddSync(&lumberjack.Logger{
		Filename:   fmt.Sprintf("%s/%s.log", logPath, appName),
		MaxSize:    100, // megabytes
		MaxBackups: 3,
		MaxAge:     7, // days
	})

	level := zap.NewAtomicLevelAt(zap.DebugLevel)

	productionCfg := zap.NewProductionEncoderConfig()
	productionCfg.TimeKey = "timestamp"
	productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	fileEncoder := zapcore.NewJSONEncoder(productionCfg)

	var logger *zap.Logger
	if !isRelease {
		developmentCfg := zap.NewDevelopmentEncoderConfig()
		developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		stdout := zapcore.AddSync(os.Stdout)
		consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)

		core := zapcore.NewTee(
			zapcore.NewCore(consoleEncoder, stdout, level),
			zapcore.NewCore(fileEncoder, file, level),
		)

		logger = zap.New(core)
	} else {
		logger = zap.New(zapcore.NewCore(fileEncoder, file, level))
	}

	logger.Info("Logger initialized", zap.String("app", appName), zap.String("vcs", buildVcsSha()))

	return logger
}

// buildVcsSha returns the VCS revision and type of the current build.
func buildVcsSha() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	vcs := "vcs"
	revision := "unknown"
	for _, kv := range info.Settings {
		if kv.Key == "vcs.revision" {
			revision = kv.Value[:8]
		}

		if kv.Key == "vcs" {
			vcs = kv.Value
		}
	}

	return fmt.Sprintf("%s:%s", vcs, revision)
}
