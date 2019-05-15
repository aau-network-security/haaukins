// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"testing"
)

type testStruct struct {
	A string
	B string
	C string
}

func TestFormatter_AsTable(t *testing.T) {
	f := formatter{
		header: []string{"A a", "B b", "C c"},
		fields: []string{"A", "B", "C"},
	}

	data := []formatElement{
		testStruct{"First", "Second", "Third"},
		testStruct{"Fourth", "Fifth", "Sixth"},
	}

	formatted, err := f.AsTable(data)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	expected := `A a      B b      C c
First    Second   Third
Fourth   Fifth    Sixth
`
	if formatted != expected {
		t.Fatalf("Expected '%s', but got '%s'", expected, formatted)
	}
}

func TestFields_Template(t *testing.T) {
	f := fields{"A", "B", "C"}

	expected := "{{.A}}\t{{.B}}\t{{.C}}"
	template := f.template()
	if template != expected {
		t.Fatalf("Unexpected output \n'%s', expected \n'%s'", template, expected)
	}
}
