package guacamole

import (
	"fmt"
	"github.com/aau-network-security/go-ntp/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"time"
)

type logEvent struct {
	timestamp time.Time
	rawFrame  RawFrame
}

type KeyLogger interface {
	Log(rm RawFrame)
}

type keyLogger struct {
	ch     chan logEvent
	logger *zerolog.Logger
	kff    KeyFrameFilter
	mff    MouseFrameFilter
}

func (k keyLogger) run() {
	for {
		event := <-k.ch

		kf, ok, err := k.kff.Filter(event.rawFrame)
		if err != nil {
			log.Warn().Msgf("Failed to filter raw message: %s", err)
		} else if ok {
			k.logger.Log().
				Time("t", event.timestamp).
				Str("k", string(kf.Key)).
				Str("p", string(kf.Pressed))
			continue
		}

		mf, ok, err := k.mff.Filter(event.rawFrame)
		if err != nil {
			log.Warn().Msgf("Failed to filter raw message: %s", err)
		} else if ok {
			k.logger.Log().
				Time("t", event.timestamp).
				Str("x", string(mf.X)).
				Str("y", string(mf.Y)).
				Str("b", string(mf.Button))
		}
	}
}

func (k keyLogger) Log(rm RawFrame) {
	timestamp := time.Now()

	k.ch <- logEvent{
		timestamp: timestamp,
		rawFrame:  rm,
	}
}

func NewKeyLogger(path string) (KeyLogger, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	logger := zerolog.New(f)

	c := make(chan logEvent)
	kl := keyLogger{
		ch:     c,
		logger: &logger,
		kff:    KeyFrameFilter{},
		mff:    MouseFrameFilter{},
	}
	go kl.run()
	return kl, nil
}

type KeyLoggerPool interface {
	GetLogger(t store.Team) (KeyLogger, error)
}

type keyLoggerPool struct {
	dir        string
	keyloggers map[string]KeyLogger
}

func (klp keyLoggerPool) addLogger(t store.Team) error {
	fn := fmt.Sprintf("%s.log", t.Id)
	fp := filepath.Join(klp.dir, fn)
	kl, err := NewKeyLogger(fp)
	if err != nil {
		return err
	}
	klp.keyloggers[t.Id] = kl
	return nil
}

func (klp keyLoggerPool) GetLogger(t store.Team) (KeyLogger, error) {
	if _, ok := klp.keyloggers[t.Id]; !ok {
		if err := klp.addLogger(t); err != nil {
			return nil, err
		}
	}
	return klp.keyloggers[t.Id], nil
}

func NewKeyLoggerPool(dir string) KeyLoggerPool {
	return keyLoggerPool{
		keyloggers: make(map[string]KeyLogger),
		dir:        dir,
	}
}
