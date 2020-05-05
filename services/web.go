package services

import (
	"fmt"
	"io"
	"net"
	"net/http"

	logrusmiddleware "github.com/bakins/logrus-middleware"
	joonix "github.com/joonix/log"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Web struct {
	pool *SRT2VTTPool
	host string
	port int
	acao string
	ln   net.Listener
}

func NewWeb(pool *SRT2VTTPool, host string, port int, acao string) *Web {
	return &Web{pool: pool, host: host, port: port, acao: acao}
}

func (s *Web) Serve() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "Failed to listen to tcp connection")
	}
	s.ln = ln
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := r.Header.Get("X-Source-Url")
		if url == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("Origin") != "" && r.Header.Get("X-CORS-Set") != "true" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Origin", s.acao)
		}
		data, err := s.pool.Get(url)
		if err != nil {
			log.WithError(err).Errorf("Failed to process request with url=%s", url)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		io.WriteString(w, data)
	})
	logger := log.New()
	logger.SetFormatter(joonix.NewFormatter())
	l := logrusmiddleware.Middleware{
		Logger: logger,
	}
	log.Infof("Serving Web at %v", addr)
	return http.Serve(ln, l.Handler(mux, ""))
}

func (s *Web) Close() {
	if s.ln != nil {
		s.ln.Close()
	}
}
