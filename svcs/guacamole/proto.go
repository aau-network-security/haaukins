package guacamole

import (
	"bytes"
	"github.com/pkg/errors"
	"strings"
	"unicode/utf8"
)

var (
	FilterErr        = errors.New("opcode is filtered")
	MalformedMsgErr  = errors.New("message is malformed")
	InvalidOpcodeErr = errors.New("invalid opcode")
	InvalidArgsErr   = errors.New("invalid number of args")

	keyOpcode   = []byte("3.key")
	mouseOpcode = []byte("5.mouse")
)

type RawFrame []byte

type Element string

func (e Element) String() string {
	return string(e)
}

type Frame struct {
	opcode Element
	args   []Element
}

func (f *Frame) String() string {
	s := []string{
		f.opcode.String(),
	}
	for _, arg := range f.args {
		s = append(s, arg.String())
	}

	return strings.Join(s, ",") + ";"
}

func NewFrame(rawFrame RawFrame) (*Frame, error) {
	msg := &Frame{
		args: []Element{},
	}

	s := ""
	for utf8.RuneCount(rawFrame) > 0 {
		r, size := utf8.DecodeRune(rawFrame)
		if r == 46 {
			// end of length
			s = ""
		} else if r == 44 || r == 59 {
			// end of element || end of message
			if msg.opcode == "" {
				msg.opcode = Element(s)
			} else {
				msg.args = append(msg.args, Element(s))
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
	if f.opcode != "key" {
		return nil, InvalidOpcodeErr
	}
	if len(f.args) != 2 {
		return nil, InvalidArgsErr
	}
	return &KeyFrame{
		Key:     f.args[0],
		Pressed: f.args[1],
	}, nil
}

type KeyFrameFilter struct{}

func (kff *KeyFrameFilter) Filter(rawFrame RawFrame) (*KeyFrame, bool, error) {
	h := []byte(rawFrame)[:len(keyOpcode)]
	if bytes.Compare(keyOpcode, h) != 0 {
		return nil, false, nil
	}
	f, err := NewFrame(rawFrame)
	if err != nil {
		return nil, false, err
	}
	kf, err := NewKeyFrame(f)
	if err != nil {
		return nil, false, err
	}
	return kf, true, nil
}

type MouseFrame struct {
	X      Element
	Y      Element
	Button Element
}

func NewMouseFrame(f *Frame) (*MouseFrame, error) {
	if f.opcode != "mouse" {
		return nil, InvalidOpcodeErr
	}
	if len(f.args) != 3 {
		return nil, InvalidArgsErr
	}

	return &MouseFrame{
		X:      f.args[0],
		Y:      f.args[1],
		Button: f.args[2],
	}, nil
}

type MouseFrameFilter struct{}

func (ff *MouseFrameFilter) Filter(rawFrame RawFrame) (*MouseFrame, bool, error) {
	h := []byte(rawFrame)[:len(mouseOpcode)]
	if bytes.Compare(mouseOpcode, h) != 0 {
		return nil, false, nil
	}
	f, err := NewFrame(rawFrame)
	if err != nil {
		return nil, false, err
	}
	mf, err := NewMouseFrame(f)
	if err != nil {
		return nil, false, err
	}
	return mf, true, nil
}
