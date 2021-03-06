package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh/terminal"

	// "encoding/json"
	log "github.com/sirupsen/logrus"
)

const (
	nocolor = 0
	red     = 31
	green   = 32
	yellow  = 33
	blue    = 36
	gray    = 37
)

var (
	baseTimestamp time.Time
)

func init() {
	baseTimestamp = time.Now()
}

// ChannelFormatter formats logs into text
type ChannelTextFormatter struct {
	// Set to true to bypass checking for a TTY before outputting colors.
	ForceColors bool

	// Force disabling colors.
	DisableColors bool

	// Disable timestamp logging. useful when output is redirected to logging
	// system that already adds timestamps.
	DisableTimestamp bool

	// Enable logging the full timestamp when a TTY is attached instead of just
	// the time passed since beginning of execution.
	FullTimestamp bool

	// TimestampFormat to use for display when a full timestamp is printed
	TimestampFormat string

	// The fields are sorted by default for a consistent output. For applications
	// that log extremely frequently and don't use the JSON formatter this may not
	// be desired.
	DisableSorting bool

	// QuoteEmptyFields will wrap empty fields in quotes if true
	QuoteEmptyFields bool

	// Whether the logger's out is to a terminal
	isTerminal bool

	sync.Once
}

func (f *ChannelTextFormatter) init(entry *log.Entry) {
	if entry.Logger != nil {
		f.isTerminal = f.checkIfTerminal(entry.Logger.Out)
	}
}

func (f *ChannelTextFormatter) checkIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return terminal.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

// Format renders a single log entry
func (f *ChannelTextFormatter) Format(entry *log.Entry) ([]byte, error) {
	var b *bytes.Buffer
	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}

	if !f.DisableSorting {
		sort.Strings(keys)
	}

	b = &bytes.Buffer{}

	// prefixFieldClashes(entry.Data)

	f.Do(func() { f.init(entry) })

	isColored := (f.ForceColors || f.isTerminal) && !f.DisableColors

	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}
	if isColored {
		f.printColored(b, entry, keys, timestampFormat)
	} else {
		if !f.DisableTimestamp {
			f.appendKeyValue(b, "time", entry.Time.Format(timestampFormat))
		}
		f.appendKeyValue(b, "level", entry.Level.String())
		if entry.Message != "" {
			f.appendKeyValue(b, "msg", entry.Message)
		}
		for _, key := range keys {
			f.appendKeyValue(b, key, entry.Data[key])
		}
	}

	// f.appendKeyValue(b, "test", "tesssssst")
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *ChannelTextFormatter) printColored(b *bytes.Buffer, entry *log.Entry, keys []string, timestampFormat string) {
	var levelColor int
	switch entry.Level {
	case log.DebugLevel:
		levelColor = gray
	case log.WarnLevel:
		levelColor = yellow
	case log.ErrorLevel, log.FatalLevel, log.PanicLevel:
		levelColor = red
	default:
		levelColor = blue
	}

	levelText := strings.ToUpper(entry.Level.String())[0:4]

	if entry.Level <= log.WarnLevel {
		if f.DisableTimestamp {
			fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m ", levelColor, levelText)
		} else if !f.FullTimestamp {
			fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%04d] ", levelColor, levelText, int(entry.Time.Sub(baseTimestamp)/time.Second))
		} else {
			fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s] ", levelColor, levelText, entry.Time.Format(timestampFormat))
		}
		for _, k := range keys {
			v := entry.Data[k]
			fmt.Fprintf(b, " \x1b[%dm%s\x1b[0m=", levelColor, k)
			f.appendValue(b, v)
		}
	} else {
		if f.DisableTimestamp {
			fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m %-44s ", levelColor, levelText, entry.Message)
		} else if !f.FullTimestamp {
			fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%04d] %-44s ", levelColor, levelText, int(entry.Time.Sub(baseTimestamp)/time.Second), entry.Message)
		} else {
			fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s] %-44s ", levelColor, levelText, entry.Time.Format(timestampFormat), entry.Message)
		}
		for _, k := range keys {
			v := entry.Data[k]
			fmt.Fprintf(b, " \x1b[%dm%s\x1b[0m=", levelColor, k)
			f.appendValue(b, v)
		}
	}

}

func (f *ChannelTextFormatter) needsQuoting(text string) bool {
	if f.QuoteEmptyFields && len(text) == 0 {
		return true
	}
	for _, ch := range text {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '.' || ch == '_' || ch == '/' || ch == '@' || ch == '^' || ch == '+') {
			return true
		}
	}
	return false
}

func (f *ChannelTextFormatter) appendKeyValue(b *bytes.Buffer, key string, value interface{}) {
	if b.Len() > 0 {
		b.WriteByte(' ')
	}
	b.WriteString(key)
	b.WriteByte('=')
	f.appendValue(b, value)
}

func (f *ChannelTextFormatter) appendValue(b *bytes.Buffer, value interface{}) {
	stringVal, ok := value.(string)
	if !ok {
		stringVal = fmt.Sprint(value)
	}

	if !f.needsQuoting(stringVal) {
		b.WriteString(stringVal)
	} else {
		b.WriteString(fmt.Sprintf("%q", stringVal))
	}
}
