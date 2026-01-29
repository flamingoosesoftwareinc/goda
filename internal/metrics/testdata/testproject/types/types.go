package types

import "testproject/base"

// Request represents an incoming request.
type Request struct {
	Config base.Config
	Body   []byte
}

// Response represents an outgoing response.
type Response struct {
	Status int
	Body   []byte
}

// Error represents a typed error.
type Error struct {
	Code    int
	Message string
}
