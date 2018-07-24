package ntp

type VirtualInstance interface {
	Start() error
	Stop() error
	Restart() error
}

type ResourceResizer interface {
	SetRAM(uint) error
	SetCPU(uint) error
}
