package virtual

type Instance interface {
	Start() error
	Kill() error
}

type ResourceResizer interface {
	SetRAM(uint) error
	SetCPU(uint) error
}

type ID string
