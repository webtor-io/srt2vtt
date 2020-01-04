package services

import (
	"fmt"
	"net"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Probe struct {
	host string
	port int
	ln   net.Listener
}

func NewProbe(host string, port int) *Probe {
	return &Probe{host: host, port: port}
}

func (s *Probe) Serve() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "Failed to listen to tcp connection")
	}
	s.ln = ln
	mux := http.NewServeMux()
	mux.HandleFunc("/liveness", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	log.Infof("Serving Probe at %v", addr)
	return http.Serve(ln, mux)
}

func (s *Probe) Close() {
	if s.ln != nil {
		s.ln.Close()
	}
}
