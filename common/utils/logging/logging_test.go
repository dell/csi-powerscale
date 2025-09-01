/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

/*
 Copyright (c) 2021-2025 Dell Inc, or its subsidiaries.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package logging

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestGetMessageWithRunID(t *testing.T) {
	tests := []struct {
		name     string
		runid    string
		format   string
		args     []interface{}
		expected string
	}{
		{"Basic message", "12345", "Process started", nil, " runid=12345 Process started"},
		{"Formatted message", "98765", "Error code: %d", []interface{}{404}, " runid=98765 Error code: 404"},
		{"Multiple arguments", "56789", "User %s logged in at %s", []interface{}{"Alice", "10:00 AM"}, " runid=56789 User Alice logged in at 10:00 AM"},
		{"Empty runID", "", "System rebooting", nil, " runid= System rebooting"},
		{"Empty format", "54321", "", nil, " runid=54321 "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetMessageWithRunID(tt.runid, tt.format, tt.args...)
			if got != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestLogMap(_ *testing.T) {
	ctx := context.Background()
	m := map[string]string{"key1": "value1", "key2": "value2"}
	LogMap(ctx, "testMap", m) // Ensure this runs without panic
}

func TestParseLogLevel(t *testing.T) {
	type args struct {
		lvl string
	}
	tests := []struct {
		name    string
		args    args
		want    logrus.Level
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				lvl: "info",
			},
			want:    logrus.InfoLevel,
			wantErr: false,
		},
		{
			name: "fail",
			args: args{
				lvl: "test",
			},
			want:    logrus.PanicLevel,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLogLevel(tt.args.lvl)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateLogLevel(t *testing.T) {
	singletonLog = logrus.New()

	type args struct {
		lvl logrus.Level
		mu  *sync.Mutex
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "update log level",
			args: args{
				lvl: logrus.ErrorLevel,
				mu:  &sync.Mutex{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateLogLevel(tt.args.lvl, tt.args.mu)
			if singletonLog.Level != tt.args.lvl {
				t.Errorf("Expected log level %v, got %v", tt.args.lvl, singletonLog.Level)
			}
		})
	}
}

func TestGetCurrentLogLevel(t *testing.T) {
	singletonLog = logrus.New()
	singletonLog.SetLevel(logrus.InfoLevel)

	tests := []struct {
		name string
		want logrus.Level
	}{
		{
			name: "return current log level",
			want: logrus.InfoLevel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCurrentLogLevel(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCurrentLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatter_Format(t *testing.T) {
	type fields struct {
		TimestampFormat  string
		LogFormat        string
		CallerPrettyfier func(*runtime.Frame) (function string, file string)
	}
	type args struct {
		entry *logrus.Entry
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "has keyed data",
			fields: fields{
				TimestampFormat:  "",
				LogFormat:        "%time%, %lvl%, %stringkey%, %intkey%, %boolkey%, %msg%, %runid%, %clusterName%",
				CallerPrettyfier: nil,
			},
			args: args{
				entry: &logrus.Entry{
					Time:    time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
					Level:   logrus.InfoLevel,
					Message: "some message",
					Data: logrus.Fields{
						RunID:       "",
						ClusterName: "some-cluster",
						"stringkey": "stringvalue",
						"intkey":    1,
						"boolkey":   true,
					},
				},
			},
			want:    []byte("1970-01-01T00:00:00Z, info, stringvalue, 1, true, some message, runid=, clusterName=some-cluster\n"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Formatter{
				TimestampFormat:  tt.fields.TimestampFormat,
				LogFormat:        tt.fields.LogFormat,
				CallerPrettyfier: tt.fields.CallerPrettyfier,
			}
			got, err := f.Format(tt.args.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("Formatter.Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Formatter.Format() = \"%v\", want \"%v\"", string(got), string(tt.want))
			}
		})
	}
}
