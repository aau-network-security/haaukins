package errors

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"runtime"
)

type Error struct {
	function FCall
	Err      error
	Severity zerolog.Level
	Msg      string
}
type FCall string

func (o FCall) String() string {
	return string(o)
}

func E(functionCall FCall, args ...interface{}) error {
	e := Error{function: functionCall}
	if len(args) == 0 {
		msg := "errors.E called with 0 args"
		_, file, line, ok := runtime.Caller(1)
		if ok {
			msg = fmt.Sprintf("%v - %v:%v", msg, file, line)
		}
		e.Err = errors.New(msg)
	}
	for _, a := range args {
		switch a := a.(type) {
		case error:
			e.Err = a
		case string:
			e.Msg = a
		case zerolog.Level:
			e.Severity = a
		}
	}
	if e.Err == nil && e.Msg != "" {
		return errors.New(e.Msg)
	}
	if e.Err != nil && e.Msg != "" {
		return errors.New(e.Error() + " " + e.Msg)
	}
	return e
}

func (e Error) Error() string {
	return e.Err.Error()
}

func Severity(err error) zerolog.Level {
	e, ok := err.(Error)
	if !ok {
		return zerolog.ErrorLevel
	}

	if e.Severity < zerolog.ErrorLevel {
		return Severity(e.Err)
	}

	return e.Severity
}
