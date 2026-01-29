package base

// Reader is an abstract reader interface.
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer is an abstract writer interface.
type Writer interface {
	Write(p []byte) (n int, err error)
}

// Config holds base configuration.
type Config struct {
	Name    string
	Verbose bool
}
