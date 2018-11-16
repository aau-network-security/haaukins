package guacamole

import (
	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/util"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

func (k keyLogger) Log(rawFrame RawFrame) {
	timestamp := time.Now()

	k.ch <- logEvent{
		timestamp: timestamp,
		rawFrame:  rawFrame,
	}
}

func NewKeyLogger(logger *zerolog.Logger) (KeyLogger, error) {
	c := make(chan logEvent)
	kl := keyLogger{
		ch:     c,
		logger: logger,
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
	logpool util.LogPool
}

func (klp keyLoggerPool) GetLogger(t store.Team) (KeyLogger, error) {
	logger, err := klp.logpool.GetLogger(t.Id)
	if err != nil {
		return nil, err
	}
	return NewKeyLogger(logger)
}

func NewKeyLoggerPool(dir string) (KeyLoggerPool, error) {
	logpool, err := util.NewLogPool(dir)
	if err != nil {
		return nil, err
	}

	return keyLoggerPool{
		logpool: logpool,
	}, nil
}
