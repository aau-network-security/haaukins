// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package store

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	tagRawRegexp = `^[a-z0-9][a-z0-9-]*[a-z0-9]$`
	tagRegex     = regexp.MustCompile(tagRawRegexp)
	TagEmptyErr  = errors.New("Tag cannot be empty")
)

type InvalidTagSyntaxErr struct {
	tag string
}

func (ite *InvalidTagSyntaxErr) Error() string {
	return fmt.Sprintf("Invalid syntax for tag \"%s\", allowed syntax: %s", ite.tag, tagRawRegexp)
}

type EmptyVarErr struct {
	Var  string
	Type string
}

func (eve *EmptyVarErr) Error() string {
	if eve.Type == "" {
		return fmt.Sprintf("%s cannot be empty", eve.Var)
	}

	return fmt.Sprintf("%s cannot be empty for %s", eve.Var, eve.Type)
}

type Tag string

func NewTag(s string) (Tag, error) {
	t := Tag(s)
	if err := t.Validate(); err != nil {
		return "", err
	}

	return t, nil
}

func (t Tag) Validate() error {
	s := string(t)
	if s == "" {
		return TagEmptyErr
	}

	if !tagRegex.MatchString(s) {
		return &InvalidTagSyntaxErr{s}
	}

	return nil
}
