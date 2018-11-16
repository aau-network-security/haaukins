package util

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
)

type LogPool interface {
	GetLogger(string, ...loggingOpts) (*zerolog.Logger, error)
	io.Closer
}

type logPool struct {
	m     sync.Mutex
	dir   string
	logs  map[string]*zerolog.Logger
	files []io.Closer
}

type logConfig struct {
	writeStdErr bool
}

type loggingOpts func(*logConfig) error

func (lp *logPool) GetLogger(name string, opts ...loggingOpts) (*zerolog.Logger, error) {
	lp.m.Lock()
	defer lp.m.Unlock()

	var conf logConfig
	for _, opt := range opts {
		if err := opt(&conf); err != nil {
			return nil, err
		}
	}

	log, ok := lp.logs[name]
	if ok {
		return log, nil
	}

	var w io.Writer

	path := filepath.Join(lp.dir, name+".log")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	w = f
	if conf.writeStdErr {
		w = io.MultiWriter(f, os.Stderr)
	}

	lp.files = append(lp.files, f)
	logger := zerolog.New(w).With().Timestamp().Logger()

	lp.logs[name] = &logger

	return &logger, nil
}

func (lp *logPool) Close() error {
	var errs error
	for _, f := range lp.files {
		err := f.Close()
		if err != nil && errs == nil {
			errs = err
		}
	}

	return errs
}

func NewLogPool(dir string) (LogPool, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, err
		}
	}

	return &logPool{
		logs: map[string]*zerolog.Logger{},
		dir:  dir,
	}, nil
}
