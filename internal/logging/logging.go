package logging

import "github.com/sirupsen/logrus"

func Logger() *logrus.Logger {
	return logrus.StandardLogger()
}
