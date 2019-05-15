// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package guacamole

import (
	"github.com/pkg/errors"
	"unicode/utf8"
)

var (
	InvalidOpcodeErr = errors.New("invalid opcode")
	InvalidArgsErr   = errors.New("invalid number of args")
)

type RawFrame []byte

type Element string

type Frame struct {
	Opcode Element
	Args   []Element
}

func NewFrame(rawFrame RawFrame) (*Frame, error) {
	msg := &Frame{
		Args: []Element{},
	}

	s := ""
	for utf8.RuneCount(rawFrame) > 0 {
		r, size := utf8.DecodeRune(rawFrame)
		if r == 46 {
			// end of length
			s = ""
		} else if r == 44 || r == 59 {
			// end of element || end of message
			if msg.Opcode == "" {
				msg.Opcode = Element(s)
			} else {
				msg.Args = append(msg.Args, Element(s))
			}
			s = ""
		} else {
			// normal rune
			s += string(r)
		}
		rawFrame = rawFrame[size:]
	}
	return msg, nil
}

type KeyFrame struct {
	Key     Element
	Pressed Element
}

func NewKeyFrame(f *Frame) (*KeyFrame, error) {
	if f.Opcode != "key" {
		return nil, InvalidOpcodeErr
	}
	if len(f.Args) != 2 {
		return nil, InvalidArgsErr
	}
	return &KeyFrame{
		Key:     f.Args[0],
		Pressed: f.Args[1],
	}, nil
}

type MouseFrame struct {
	X      Element
	Y      Element
	Button Element
}

func NewMouseFrame(f *Frame) (*MouseFrame, error) {
	if f.Opcode != "mouse" {
		return nil, InvalidOpcodeErr
	}
	if len(f.Args) != 3 {
		return nil, InvalidArgsErr
	}

	return &MouseFrame{
		X:      f.Args[0],
		Y:      f.Args[1],
		Button: f.Args[2],
	}, nil
}
