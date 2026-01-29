package handler

import (
	"testproject/base"
	"testproject/service"
)

// Server handles requests using a service registry.
type Server struct {
	Registry service.Registry
	Config   base.Config
}

// Client sends requests to a server.
type Client struct {
	Target string
}
