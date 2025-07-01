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
	"testing"
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
