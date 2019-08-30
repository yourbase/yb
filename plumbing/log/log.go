package log

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

var (
	log       *logrus.Logger
	level     logrus.Level
	formatter = NewYbFormatter()
)

type YbFormatter struct {
	logrus.TextFormatter
	Section    string
	LogSection bool
}

func NewYbFormatter() *YbFormatter {
	return &YbFormatter{
		Section:    "",
		LogSection: false,
	}
}

func init() {
	log = logrus.New()
	// TODO use settings/config to set another default level
	level = logrus.InfoLevel
	log.SetLevel(level)
	log.SetOutput(os.Stderr)
	log.SetFormatter(formatter)
}
func ActiveSection(section string) {
	formatter.Section = section
}

func (f *YbFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	prefix := ""
	if f.LogSection {
		s := f.Section
		if len(s) > 3 {
			s = s[0:3]
		}
		prefix = fmt.Sprintf("[%3s] ", strings.ToUpper(s))
	}
	message := fmt.Sprintf("%s%s\n", prefix, entry.Message)
	return []byte(message), nil
}

func SetLevel(l logrus.Level) { level = l }

func Logf(level logrus.Level, format string, args ...interface{}) { log.Logf(level, format, args...) }

func Tracef(format string, args ...interface{}) { Logf(logrus.TraceLevel, format, args...) }

func Debugf(format string, args ...interface{}) { Logf(logrus.DebugLevel, format, args...) }

func Infof(format string, args ...interface{}) { Logf(logrus.InfoLevel, format, args...) }

func Warnf(format string, args ...interface{}) { Logf(logrus.WarnLevel, format, args...) }

func Warningf(format string, args ...interface{}) { Warnf(format, args...) }

func Errorf(format string, args ...interface{}) { Logf(logrus.ErrorLevel, format, args...) }

func Fatalf(format string, args ...interface{}) { Logf(logrus.FatalLevel, format, args...); log.Exit(1) }

func Panicf(format string, args ...interface{}) { Logf(logrus.PanicLevel, format, args...) }

func Log(level logrus.Level, args ...interface{}) { log.Log(level, args...) }

func Trace(args ...interface{}) { Log(logrus.TraceLevel, args...) }

func Debug(args ...interface{}) { Log(logrus.DebugLevel, args...) }

func Info(args ...interface{}) { Log(logrus.InfoLevel, args...) }

func Warn(args ...interface{}) { Log(logrus.WarnLevel, args...) }

func Warning(args ...interface{}) { Warn(args...) }

func Error(args ...interface{}) { Log(logrus.ErrorLevel, args...) }

func Fatal(args ...interface{}) { Log(logrus.FatalLevel, args...); log.Exit(1) }

func Panic(args ...interface{}) { Log(logrus.PanicLevel, args...) }

func Logln(level logrus.Level, args ...interface{}) { log.Logln(level, args...) }

func Traceln(args ...interface{}) { Logln(logrus.TraceLevel, args...) }

func Debugln(args ...interface{}) { Logln(logrus.DebugLevel, args...) }

func Infoln(args ...interface{}) { Logln(logrus.InfoLevel, args...) }

func Warnln(args ...interface{}) { Logln(logrus.WarnLevel, args...) }

func Warningln(args ...interface{}) { Warnln(args...) }

func Errorln(args ...interface{}) { Logln(logrus.ErrorLevel, args...) }

func Fatalln(args ...interface{}) { Logln(logrus.FatalLevel, args...); log.Exit(1) }

func Panicln(args ...interface{}) { Logln(logrus.PanicLevel, args...) }
