package log

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/sirupsen/logrus"
	ybconfig "github.com/yourbase/yb/config"
)

var (
	log       *logrus.Logger
	level     logrus.Level
	Formatter = NewYbFormatter()
)

func init() {
	log = logrus.New()

	sLevel, _ := ybconfig.GetConfigValue("defaults", "log-level")
	level, err := logrus.ParseLevel(sLevel)
	if err == nil {
		log.SetLevel(level)
	}
	out, _ := ybconfig.GetConfigValue("defaults", "no-pretty-output")
	Formatter.NoPrettyOut = out == "true"

	if out, exists := os.LookupEnv("YB_NO_PRETTY_OUTPUT"); exists {
		Formatter.NoPrettyOut = out == "true"
	}

	log.SetFormatter(Formatter)
}

type YbFormatter struct {
	logrus.TextFormatter
	Section        string
	LogSection     bool
	NoPrettyOut    bool
	innerFormatter *logrus.TextFormatter
}

func NewYbFormatter() *YbFormatter {
	ft := &YbFormatter{
		Section:    "",
		LogSection: false,
		innerFormatter: &logrus.TextFormatter{
			DisableTimestamp: true,

			// from https://bixense.com/clicolors/
			// Set CLICOLOR to 0 to force plain "black/white" text colors
			EnvironmentOverrideColors: true,
		},
	}

	return ft
}

func StartSection(name, section string) {
	ActiveSection(section)
	Formatter.LogSection = true
	fmt.Printf("\n === %s ===\n\n", name)
}

func SubSection(name string) {
	fmt.Printf("\n -- %s -- \n\n", name)
}

func ActiveSection(section string) {
	Formatter.Section = section
}

func EndSection() {
	Formatter.Section = ""
	Formatter.LogSection = false
}

func Title(t string) {
	fmt.Println(strings.ToUpper(t))
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

	entry.Message = fmt.Sprintf("%s%s\n", prefix, entry.Message)
	if !f.NoPrettyOut && checkIfTerminal(log.Out) {
		return f.innerFormatter.Format(entry)
	}
	// Plain old boring logging
	return []byte(entry.Message), nil
}

func checkIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return terminal.IsTerminal(int(v.Fd()))
	default:
		return false
	}
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
