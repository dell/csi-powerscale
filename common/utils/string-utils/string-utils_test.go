package stringutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsStringInSlice(t *testing.T) {
	list := []string{"hello", "world", "jason"}

	assert.True(t, IsStringInSlice("world", list))
	assert.False(t, IsStringInSlice("mary", list))
	assert.False(t, IsStringInSlice("harry", nil))
}

func TestRemoveStringFromSlice(t *testing.T) {
	list := []string{"hello", "world", "jason"}

	result := RemoveStringFromSlice("hello", list)

	assert.Equal(t, 3, len(list))
	assert.Equal(t, 2, len(result))

	result = RemoveStringFromSlice("joe", list)

	assert.Equal(t, 3, len(result))
}

func TestRemoveStringsFromSlice(t *testing.T) {
	list := []string{"hello", "world", "jason"}

	filterList := []string{"hello", "there", "chap", "world"}

	result := RemoveStringsFromSlice(filterList, list)

	assert.Equal(t, 1, len(result))
}

func TestRemoveSurroundingQuotes(t *testing.T) {
	// Case 1: String with quotes at both ends
	input := `"hello"`
	expected := "hello"
	result := RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected quotes to be removed")

	// Case 2: String with quotes only at the beginning
	input = `"hello`
	expected = "hello"
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected leading quote to be removed")

	// Case 3: String with quotes only at the end
	input = `hello"`
	expected = "hello"
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected trailing quote to be removed")

	// Case 4: String without quotes
	input = `hello`
	expected = "hello"
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected string to remain unchanged")

	// Case 5: Empty string
	input = ``
	expected = ``
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected empty string to remain unchanged")

	// Case 6: String with only one quote character (should be removed)
	input = `"`
	expected = ``
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected single quote to be removed")

	// Case 7: String with multiple words and surrounding quotes
	input = `"hello world"`
	expected = "hello world"
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected surrounding quotes to be removed")

	// Case 8: String with nested quotes (should only remove outermost quotes)
	input = `""hello""`
	expected = `"hello"`
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected only outermost quotes to be removed")

	// Case 9: String with space and quotes at both ends
	input = `" hello "`
	expected = " hello "
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected surrounding quotes to be removed but preserve spaces")

	// Case 10: String that is just two quotes (should become empty)
	input = `""`
	expected = ``
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected two quotes to be removed and return empty string")
}

func TestCombineTwoStrings(t *testing.T) {
	// Case 1: Normal strings with a hyphen as a separator
	result := CombineTwoStrings("hello", "world", "-")
	expected := "hello-world"
	assert.Equal(t, expected, result, "Expected 'hello-world'")

	// Case 2: Empty first string
	result = CombineTwoStrings("", "world", "-")
	expected = "-world"
	assert.Equal(t, expected, result, "Expected '-world' when first string is empty")

	// Case 3: Empty second string
	result = CombineTwoStrings("hello", "", "-")
	expected = "hello-"
	assert.Equal(t, expected, result, "Expected 'hello-' when second string is empty")

	// Case 4: Empty separator
	result = CombineTwoStrings("hello", "world", "")
	expected = "helloworld"
	assert.Equal(t, expected, result, "Expected 'helloworld' when separator is empty")

	// Case 5: Both strings empty
	result = CombineTwoStrings("", "", "-")
	expected = "-"
	assert.Equal(t, expected, result, "Expected '-' when both strings are empty")

	// Case 6: Special characters in strings
	result = CombineTwoStrings("foo@", "bar$", "#")
	expected = "foo@#bar$"
	assert.Equal(t, expected, result, "Expected 'foo@#bar$'")

	// Case 7: Long strings
	result = CombineTwoStrings("longstring1", "longstring2", "--")
	expected = "longstring1--longstring2"
	assert.Equal(t, expected, result, "Expected 'longstring1--longstring2'")

	// Case 8: Numeric strings
	result = CombineTwoStrings("123", "456", ":")
	expected = "123:456"
	assert.Equal(t, expected, result, "Expected '123:456' for numeric strings")
}

func TestIsStringInSlices(t *testing.T) {
	// Case 1: String is present in one of the slices
	result := IsStringInSlices("apple", []string{"banana", "cherry"}, []string{"apple", "grape"})
	assert.True(t, result, "Expected 'apple' to be found in one of the slices")

	// Case 2: String is not present in any slice
	result = IsStringInSlices("mango", []string{"banana", "cherry"}, []string{"apple", "grape"})
	assert.False(t, result, "Expected 'mango' not to be found in any slice")

	// Case 3: String is present in multiple slices
	result = IsStringInSlices("apple", []string{"apple", "cherry"}, []string{"apple", "grape"})
	assert.True(t, result, "Expected 'apple' to be found in multiple slices")

	// Case 4: String is present in an empty slice
	result = IsStringInSlices("apple", []string{}, []string{"apple", "grape"})
	assert.True(t, result, "Expected 'apple' to be found even if one slice is empty")

	// Case 5: All slices are empty
	result = IsStringInSlices("apple", []string{}, []string{})
	assert.False(t, result, "Expected 'apple' not to be found in empty slices")

	// Case 6: String is present in a slice containing only itself
	result = IsStringInSlices("orange", []string{"orange"})
	assert.True(t, result, "Expected 'orange' to be found in a single-element slice")

	// Case 7: String is present in a case-sensitive manner
	result = IsStringInSlices("apple", []string{"Apple"}, []string{"aPPle"})
	assert.False(t, result, "Expected 'apple' not to be found due to case sensitivity")

	// Case 8: Empty search string
	result = IsStringInSlices("", []string{"banana", "cherry"}, []string{"apple", "grape"})
	assert.False(t, result, "Expected empty string not to be found in slices")
}
