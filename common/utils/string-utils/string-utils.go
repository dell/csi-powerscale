/*
Copyright (c) 2019-2025 Dell Inc, or its subsidiaries.

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
package stringutils

// IsStringInSlice checks if a string is an element of a string slice
func IsStringInSlice(str string, list []string) bool {
	for _, b := range list {
		if b == str {
			return true
		}
	}

	return false
}

// IsStringInSlices checks if a string is an element of a any of the string slices
func IsStringInSlices(str string, list ...[]string) bool {
	for _, strs := range list {
		if IsStringInSlice(str, strs) {
			return true
		}
	}

	return false
}

// RemoveStringFromSlice returns a slice that is a copy of the input "list" slice with the input "str" string removed
func RemoveStringFromSlice(str string, list []string) []string {
	result := make([]string, 0)

	for _, v := range list {
		if str != v {
			result = append(result, v)
		}
	}

	return result
}

// RemoveStringsFromSlice generates a slice that is a copy of the input "list" slice with elements from the input "strs" slice removed
func RemoveStringsFromSlice(filters []string, list []string) []string {
	result := make([]string, 0)

	for _, str := range list {
		if !IsStringInSlice(str, filters) {
			result = append(result, str)
		}
	}

	return result
}

// RemoveSurroundingQuotes removes the surrounding double quotes of a given string (if there are no surrounding quotes, do nothing)
func RemoveSurroundingQuotes(s string) string {
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}

	return s
}

// CombineTwoStrings combines two string variables isolated by defined sign
func CombineTwoStrings(s1 string, s2 string, sign string) string {
	s := s1 + sign + s2
	return s
}
