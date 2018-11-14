package guacamole

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	FilterErr       = errors.New("opcode is filtered")
	MalformedMsgErr = errors.New("message is malformed")
)

type Element struct {
	length int
	value  string
}

func (e *Element) String() string {
	return fmt.Sprintf("%d.%s", e.length, e.value)
}

type Message struct {
	opcode *Element
	args   []*Element
}

func (m *Message) String() string {
	s := []string{
		m.opcode.String(),
	}
	for _, arg := range m.args {
		s = append(s, arg.String())
	}

	return strings.Join(s, ",") + ";"
}

func (m *Message) ArgsString() string {
	var s []string
	for _, arg := range m.args {
		s = append(s, arg.String())
	}
	return strings.Join(s, ",") + ";"
}

func NewMessage(b []byte) (*Message, error) {
	msg := &Message{
		args: []*Element{},
	}
	el := &Element{}

	s := ""
	for utf8.RuneCount(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == 46 {
			// end of length
			l, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}
			el.length = l
			s = ""
		} else if r == 44 || r == 59 {
			// end of element || end of message
			el.value = s
			if msg.opcode == nil {
				msg.opcode = el
			} else {
				msg.args = append(msg.args, el)
			}
			el = &Element{}
			s = ""
		} else {
			// normal rune
			s += string(r)
		}
		b = b[size:]
	}
	return msg, nil
}

type MessageFilter struct {
	opcodes []string
}

func (mf *MessageFilter) Filter(b []byte) (*Message, bool, error) {
	c := append([]byte(nil), b...)

	s := ""
	for utf8.RuneCount(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == 46 {
			// end of length
			s = ""
		} else if r == 44 || r == 59 {
			// end of element
			keep := false
			for _, opcode := range mf.opcodes {
				if s == opcode {
					keep = true
					break
				}
			}
			if !keep {
				return nil, true, nil
			}
			msg, err := NewMessage(c)
			return msg, false, err
		} else {
			// normal rune
			s += string(r)
		}
		b = b[size:]
	}
	return nil, false, MalformedMsgErr
}

type logEvent struct {
	ts   time.Time
	data []byte
}

type messageLogger struct {
	logFunc func(ts time.Time, event Message)
	c       chan logEvent
	mf      MessageFilter
}

func (mp *messageLogger) log(t logEvent) {
	mp.c <- t
}

func (mp *messageLogger) run() {
	for {
		event := <-mp.c
		msg, dropped, err := mp.mf.Filter(event.data)
		if err != nil {
			log.Debug().Msgf("Failed to filter message: %s", err)
		} else if !dropped {
			mp.logFunc(event.ts, *msg)
		}
	}
}

func newMessageLogger(logFunc func(ts time.Time, event Message), filterOpcodes []string) messageLogger {
	mf := MessageFilter{
		opcodes: filterOpcodes,
	}

	mp := messageLogger{
		logFunc: logFunc,
		c:       make(chan logEvent),
		mf:      mf,
	}
	go mp.run()
	return mp
}
