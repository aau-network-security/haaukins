package errors

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"runtime"
)

type Error struct {
	function FCall
	Err      error
	Severity logrus.Level
}
func (e Error) Error() string {
	return e.Err.Error()
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
			e.Err = errors.New(a)
		case logrus.Level:
			e.Severity = a
		}
	}
	if e.Err == nil {
		e.Err = errors.New(e.Error())
	}

	return e
}

func Severity(err error) logrus.Level {
	e, ok := err.(Error)
	if !ok {
		return logrus.ErrorLevel
	}

	if e.Severity < logrus.ErrorLevel {
		return Severity(e.Err)
	}

	return e.Severity
}