package ntp

import "context"

type StartStopper interface {
	Start(context.Context) error
	Stop(context.Context) error
}

func Restart(ctx context.Context, ss StartStopper) error {
	if err := ss.Stop(ctx); err != nil {
		return err
	}

	if err := ss.Start(ctx); err != nil {
		return err
	}

	return nil
}

type CheckPointer interface {
	Points() []CheckPoint
}

type CheckPoint struct {
	Name  string
	Value string
	Score uint
}
