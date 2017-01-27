package prometheus

import (
	"io"
	"log"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// TODO: rethink when we switch to go1.8. the graceful shutdown changes
// coming to http.Server should help tremendously

type (
	server interface {
		io.Closer

		Start() error
	}

	serverInst struct {
		server

		addr       string
		connCloser io.Closer
	}
)

const (
	metricsPath = "/metrics"
	defaultAddr = ":42422"
)

func newServer(addr string) server {
	return &serverInst{addr: addr}
}

func (s *serverInst) Start() error {
	var listener net.Listener
	srv := &http.Server{Addr: s.addr}
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	http.Handle(metricsPath, promhttp.Handler())

	go func() {
		err := srv.Serve(listener.(*net.TCPListener))
		if err != nil {
			// TODO: update and fix
			log.Println("Prometheus HTTP Server Error - ", err)
		}
	}()

	s.connCloser = listener
	return nil
}

func (s *serverInst) Close() error {
	return s.connCloser.Close()
}
