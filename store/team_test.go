package store

import (
	"reflect"
	"testing"
)

func TestCopyMap(t *testing.T) {
	m1 := map[string][]string{
		"apple":  {"a", "p", "p", "l", "e"},
		"banana": {"b", "a", "n", "a", "n", "a"},
	}

	m2 := CopyMap(m1)

	m1["apple"] = []string{"n", "o", "t", "a", "p", "p", "l", "e"}
	delete(m1, "apple")
	expected := map[string][]string{"banana": {"b", "a", "n", "a", "n", "a"}}

	if !reflect.DeepEqual(m1, expected) {
		t.Fatalf("Maps are not matching as expected, Expected: %v Actual: %v ", expected, m1)
	}
	expected = map[string][]string{
		"apple":  {"a", "p", "p", "l", "e"},
		"banana": {"b", "a", "n", "a", "n", "a"},
	}
	if !reflect.DeepEqual(m2, expected) {
		t.Fatalf("Maps are not matching as expected, Expected: %v Actual: %v ", expected, m2)
	}
}
