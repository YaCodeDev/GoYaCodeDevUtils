package config

import (
	"github.com/sirupsen/logrus"
)

// safetyCheck ensures that the logger is not nil before performing any operations.
// If the logger is nil, it initializes a new logger and logs a warning message.
func safetyCheck(log *logrus.Entry) {
	if log == nil {
		log = logrus.NewEntry(logrus.StandardLogger())

		log.Warn("Logger is nil, using default logger")
	}
}
