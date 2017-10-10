package cmd

import (
	"fmt"
	"log"
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Level *zapcore.Level
var Logger *zap.Logger
var zapConfig zap.Config

func init() {
	Level = zap.LevelFlag("logLevel", zapcore.InfoLevel, "set logging level")

	zapConfig = zap.NewProductionConfig()
	zapConfig.Encoding = "console"
	// config.DisableCaller = true
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapConfig.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	zapConfig.Level = zap.NewAtomicLevelAt(*Level)

	logger, err := zapConfig.Build()
	if err != nil {
		log.Fatalf("Building logging config broke: %v", err)
	}

	Logger = logger.Named("mixer-log")
}

func logsConfigFn(out http.ResponseWriter, req *http.Request) {

	if level := req.URL.Query().Get("level"); len(level) > 0 {
		zapConfig.Level.UnmarshalText([]byte(level))
	}

	out.Write([]byte(fmt.Sprintf("%#v", zapConfig)))
}
