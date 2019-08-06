package cli

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

type YbFormatter struct {
	log.TextFormatter
	Section    string
	LogSection bool
}

var (
	formatter = NewYbFormatter()
)

func NewYbFormatter() *YbFormatter {
	return &YbFormatter{
		Section:    "",
		LogSection: false,
	}
}
func SetupOutput() {
	log.SetOutput(os.Stderr)
	log.SetFormatter(formatter)
}

func ActiveSection(section string) {
	formatter.Section = section
}

func (f *YbFormatter) Format(entry *log.Entry) ([]byte, error) {
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
