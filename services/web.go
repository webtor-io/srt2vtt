package services

import (
	"fmt"
	"github.com/urfave/cli"
	"io"
	"net"
	"net/http"

	logrusmiddleware "github.com/bakins/logrus-middleware"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Web struct {
	pool *SRT2VTT
	host string
	port int
	ln   net.Listener
}

const (
	webHostFlag = "host"
	webPortFlag = "port"
)

func RegisterWebFlags(f []cli.Flag) []cli.Flag {
	return append(f,
		cli.StringFlag{
			Name:   webHostFlag,
			Usage:  "listening host",
			Value:  "",
			EnvVar: "WEB_HOST",
		},
		cli.IntFlag{
			Name:   webPortFlag,
			Usage:  "http listening port",
			Value:  8080,
			EnvVar: "WEB_PORT",
		},
	)
}

func NewWeb(c *cli.Context, pool *SRT2VTT) *Web {
	return &Web{
		pool: pool,
		host: c.String(webHostFlag),
		port: c.Int(webPortFlag),
	}
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
		data, err := s.pool.Get(url)
		if err != nil {
			log.WithError(err).Errorf("Failed to process request with url=%s", url)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, data)
	})
	logger := log.New()
	logger.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	l := logrusmiddleware.Middleware{
		Logger: logger,
	}
	log.Infof("Serving Web at %v", addr)
	srv := &http.Server{
		Handler: l.Handler(mux, ""),
		// ReadTimeout:    5 * time.Minute,
		// WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 50 << 20,
	}
	return srv.Serve(ln)
}

func (s *Web) Close() {
	if s.ln != nil {
		_ = s.ln.Close()
	}
}
