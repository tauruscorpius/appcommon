package Log

import (
	"github.com/tauruscorpius/logrus"
	"os"
	"runtime"
	"strconv"
	"strings"
)

var l = logrus.New()

var (
	WithField  = l.WithField
	WithFields = l.WithFields
	Trace      = l.Trace
	Tracef     = l.Tracef
	Debug      = l.Debugf
	Debugf     = l.Debugf
	Info       = l.Info
	Infof      = l.Infof
	Warn       = l.Warn
	Warnf      = l.Warnf
	Error      = l.Error
	Errorf     = l.Errorf
	Fatal      = l.Fatal
	Fatalf     = l.Fatalf
	Panic      = l.Panic
	Panicf     = l.Panicf
	Critical   = l.Critical
	Criticalf  = l.Criticalf
)

// SetLogLevel -
// enabled Log Level
// 0: Error
// 1: Critical
// 2: Warning
// 3: Info
// 4: Debug
// 5: Trace
func SetLogLevel(level int) {
	switch level {
	case 0:
		l.SetLevel(logrus.ErrorLevel)
	case 1:
		l.SetLevel(logrus.CriticalLevel)
	case 2:
		l.SetLevel(logrus.WarnLevel)
	case 3:
		l.SetLevel(logrus.InfoLevel)
	case 4:
		l.SetLevel(logrus.DebugLevel)
	case 5:
		l.SetLevel(logrus.TraceLevel)
	default:
		Critical("unknown log level [%d], default close loglevel\n", level)
		CloseLogLevel()
	}
}

func CloseLogLevel() {
	l.SetLevel(logrus.CriticalLevel)
}

func init() {
	Init()
}

func Init() {
	l.SetReportCaller(true)
	trim := func(in string, delimiter byte) string {
		loc := strings.LastIndexByte(in, delimiter)
		if loc < 0 {
			return in
		}
		return in[loc+1:]
	}
	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(r *runtime.Frame) (function string, file string) {
			return trim(r.Function, '/'), trim(r.File, '/') + ":" + strconv.Itoa(r.Line)
		}}
	l.SetFormatter(formatter)
	l.SetLevel(logrus.TraceLevel)
	l.SetOutput(os.Stdout)
	go ArchiveLogFiles()
}
