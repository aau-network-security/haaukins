// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package logging

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
)

type Pool interface {
	GetLogger(string, ...loggingOpts) (*zerolog.Logger, error)
	io.Closer
}

type GrpcLogging interface {
	Msg(msg string) error
}

func LoggerFromCtx(ctx context.Context) GrpcLogging {
	val := ctx.Value("grpc_logger")
	if val == nil {
		return nil
	}
	l, ok := val.(GrpcLogging)
	if !ok {
		return nil
	}
	return l
}

type pool struct {
	m     sync.Mutex
	dir   string
	logs  map[string]*zerolog.Logger
	files []io.Closer
}

type logConfig struct {
	writeStdErr bool
}

type loggingOpts func(*logConfig) error

func (lp *pool) GetLogger(name string, opts ...loggingOpts) (*zerolog.Logger, error) {
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
	logger := zerolog.New(w)

	lp.logs[name] = &logger

	return &logger, nil
}

func (lp *pool) Close() error {
	var errs error
	for _, f := range lp.files {
		err := f.Close()
		if err != nil && errs == nil {
			errs = err
		}
	}

	return errs
}

func NewPool(dir string) (Pool, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, err
		}
	}

	return &pool{
		logs: map[string]*zerolog.Logger{},
		dir:  dir,
	}, nil
}
