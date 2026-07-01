package logger

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log  *zap.Logger
	once sync.Once
)

func InitLogger() error {
	var err error
	once.Do(func() {
		config := zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		log, err = config.Build()
	})
	return err
}

func GetLogger() *zap.Logger {
	if log == nil {
		// Inicialización de emergencia si no se ha inicializado
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		var err error
		log, err = config.Build()
		if err != nil {
			// Como último recurso, crear un logger básico
			log = zap.NewNop()
		}
	}
	return log
}
