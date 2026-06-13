package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// New creates a logrus logger configured for the given service name.
// Respects LOG_LEVEL env var (debug, info, warn, error). Default: info.
func New(service string) *logrus.Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)

	level := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch level {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "warn", "warning":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}

	return log
}
