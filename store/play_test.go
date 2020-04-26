// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package store_test

import (
	"testing"

	. "github.com/aau-network-security/haaukins/store"
)

func TestPlayValidate(t *testing.T) {
	p := Play{
		Tag:   Tag(""),
		Steps: []PlayStep{PlayStep{}},
	}

	if err := p.Validate(); err != TagEmptyErr {
		t.Fatalf("expected TagEmptyErr got %q instead", err)
	}

	var err error

	p.Steps = []PlayStep{}
	p.Tag, err = NewTag("test")
	if err != nil {
		t.Fatalf("could not create test tag")
	}

	if err := p.Validate(); err != PlayNoStepsErr {
		t.Fatalf("expected PlayNoStepsErr got %q instead", err)
	}

	p.Steps = []PlayStep{PlayStep{}}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected no error got %q instead", err)
	}
}

func TestAnonPlayValid(t *testing.T) {
	p := PlayFromExercises("test", "")

	if err := p.Validate(); err != nil {
		t.Fatalf("expected no error got %q instead", err)
	}
}

var play1 = Play{
	Tag: Tag("testa"),
	Steps: []PlayStep{
		PlayStep{},
	},
}
var play2 = Play{
	Tag: Tag("testb"),
	Steps: []PlayStep{
		PlayStep{},
	},
}

func TestPlayStore(t *testing.T) {
	plays := []Play{
		play1,
		play2,
	}
	ps := NewPlayStore(plays)

	testtag := Tag("testa")
	play, ok := ps.GetPlayByTag(testtag)
	if !ok {
		t.Fatalf("Expected testa play to be found")
	}

	if play.Tag != testtag {
		t.Fatalf("Expected tag %s to match %s", play.Tag, testtag)
	}

	testtag = Tag("testb")
	play, ok = ps.GetPlayByTag(testtag)
	if !ok {
		t.Fatalf("Expected testb play to be found")
	}

	if play.Tag != testtag {
		t.Fatalf("Expected tag %s to match %s", play.Tag, testtag)
	}

	_, ok = ps.GetPlayByTag(Tag("notatag"))
	if ok {
		t.Fatalf("Expected notatag to return error")
	}

}
