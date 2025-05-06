package configslog

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger
var SLog *zap.SugaredLogger

func InitLogger() {
	if Log != nil {
		return
	}

	env := os.Getenv("APP_ENV")
	var config zap.Config
	var err error
	var level zapcore.Level

	if env == "production" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		level = zapcore.InfoLevel
	} else {
		env = "development"
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		level = zapcore.DebugLevel
	}

	logLevelEnv := os.Getenv("LOG_LEVEL")
	if logLevelEnv != "" {
		err = level.Set(logLevelEnv)
		if err != nil {
			panic("Geçersiz LOG_LEVEL '" + logLevelEnv + "': " + err.Error())
		}
	}
	config.Level = zap.NewAtomicLevelAt(level)

	Log, err = config.Build(zap.AddCaller())
	if err != nil {
		panic("Zap logger başlatılamadı: " + err.Error())
	}

	SLog = Log.Sugar()

	SLog.Infow("Zap logger başarıyla başlatıldı",
		"environment", env,
		"log_level", config.Level.Level().String(),
	)
}

func SyncLogger() {
	if Log != nil {
		_ = Log.Sync()
	}
	if SLog != nil {
		_ = SLog.Sync()
	}
}
