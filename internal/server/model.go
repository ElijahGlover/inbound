package server

import (
	"github.com/elijahglover/inbound/internal/controller"
)

type healthcheckResponse struct {
	Services []*controller.Service
	Routes   []*controller.RouteTable
}
