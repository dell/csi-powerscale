package utils

import (
	"context"
	"fmt"
	"github.com/dell/csi-isilon/common/constants"
	"github.com/sirupsen/logrus"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var singletonLog *logrus.Logger
var once sync.Once

// LogConst represents string in context.WithValue
type LogConst string

// Constants used for logging
const (
	// Default log format will output [INFO]: 2006-01-02T15:04:05Z07:00 - Log message
	defaultLogFormat                = "time=\"%time%\" level=%lvl% %clusterName% %runid% msg=\"%msg%\""
	defaultTimestampFormat          = time.RFC3339
	ClusterName                     = "clusterName"
	PowerScaleLogger       LogConst = "powerscalelog"
	LogFields              LogConst = "fields"
	RequestID                       = "requestid"
	RunID                           = "runid"
)

// Formatter implements logrus.Formatter interface.
type Formatter struct {
	//logrus.TextFormatter
	// Timestamp format
	TimestampFormat string
	// Available standard keys: time, msg, lvl
	// Also can include custom fields but limited to strings.
	// All of fields need to be wrapped inside %% i.e %time% %msg%
	LogFormat string

	CallerPrettyfier func(*runtime.Frame) (function string, file string)
}

// Format building log message.
func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	output := f.LogFormat
	if output == "" {
		output = defaultLogFormat
	}

	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}

	output = strings.Replace(output, "%time%", entry.Time.Format(timestampFormat), 1)
	output = strings.Replace(output, "%msg%", entry.Message, 1)
	level := strings.ToUpper(entry.Level.String())
	output = strings.Replace(output, "%lvl%", strings.ToLower(level), 1)

	fields := entry.Data

	runID, ok := fields[RunID]
	if ok {
		output = strings.Replace(output, "%runid%", fmt.Sprintf("runid=%v", runID), 1)
	} else {
		output = strings.Replace(output, "%runid%", "", 1)
	}

	clusterName, ok := fields[ClusterName]
	if ok {
		output = strings.Replace(output, "%clusterName%", fmt.Sprintf("clusterName=%v", clusterName), 1)
	} else {
		output = strings.Replace(output, "%clusterName%", "", 1)
	}

	for k, val := range entry.Data {
		switch v := val.(type) {
		case string:
			output = strings.Replace(output, "%"+k+"%", v, 1)
		case int:
			s := strconv.Itoa(v)
			output = strings.Replace(output, "%"+k+"%", s, 1)
		case bool:
			s := strconv.FormatBool(v)
			output = strings.Replace(output, "%"+k+"%", s, 1)
		}
	}

	var fileVal string
	if entry.HasCaller() {
		if f.CallerPrettyfier != nil {
			_, fileVal = f.CallerPrettyfier(entry.Caller)
		} else {
			fileVal = fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line)
		}

		if fileVal != "" {
			output = fmt.Sprintf("%s file=\"%s\"", output, fileVal)
		}
	}

	output = fmt.Sprintf("%s\n", output)

	return []byte(output), nil
}

// GetLogger function to get custom logging
func GetLogger() *logrus.Logger {
	once.Do(func() {
		singletonLog = logrus.New()
		fmt.Println("csi-powerscale logger initiated. This should be called only once.")
		singletonLog.Level = constants.DefaultLogLevel
		singletonLog.SetReportCaller(true)
		singletonLog.Formatter = &Formatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				filename1 := strings.Split(f.File, "dell/csi-powerscale")
				if len(filename1) > 1 {
					return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("dell/csi-powerscale%s:%d", filename1[1], f.Line)
				}

				filename2 := strings.Split(f.File, "dell/goisilon")
				if len(filename2) > 1 {
					return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("dell/goisilon%s:%d", filename2[1], f.Line)
				}

				return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", f.File, f.Line)
			},
		}
	})

	return singletonLog
}

// GetRunIDLogger returns the current runID logger
func GetRunIDLogger(ctx context.Context) *logrus.Entry {
	tempLog := ctx.Value(PowerScaleLogger)
	if ctx.Value(PowerScaleLogger) != nil && reflect.TypeOf(tempLog) == reflect.TypeOf(&logrus.Entry{}) {
		return ctx.Value(PowerScaleLogger).(*logrus.Entry)
	}

	return GetLogger().WithFields(logrus.Fields{})
}

// ParseLogLevel returns the logrus.Level of input log level string
func ParseLogLevel(lvl string) (logrus.Level, error) {
	return logrus.ParseLevel(lvl)
}

// UpdateLogLevel updates the log level
func UpdateLogLevel(lvl logrus.Level) {
	singletonLog.Level = lvl
}

// GetCurrentLogLevel updates the log level
func GetCurrentLogLevel() logrus.Level {
	return singletonLog.Level
}
