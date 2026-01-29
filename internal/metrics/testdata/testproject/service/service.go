package service

import (
	"testproject/base"
	"testproject/types"
)

// Handler defines the service handler interface.
type Handler interface {
	Handle(req types.Request) types.Response
}

// Registry holds registered services.
type Registry struct {
	Readers []base.Reader
}
