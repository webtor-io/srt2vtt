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
	cs "github.com/webtor-io/common-services"
	"github.com/webtor-io/lazymap"
)

type Web struct {
	pool *SRT2VTT
	host string
	port int
	gs   *cs.GracefulServer
}

const (
	webHostFlag = "host"
	webPortFlag = "port"
)

func RegisterWebFlags(f []cli.Flag) []cli.Flag {
	f = cs.RegisterShutdownFlags(f)
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
		gs:   cs.NewGracefulServer(cs.ShutdownTimeout(c)),
	}
}

func (s *Web) Serve() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "failed to listen to tcp connection")
	}
	log.Infof("serving Web at %v", addr)
	return s.serve(ln)
}

func (s *Web) serve(ln net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := r.Header.Get("X-Source-Url")
		if url == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data, err := s.pool.Get(r.Context(), url)
		if err != nil {
			l := log.WithError(err)
			// A panic in the conversion comes back as an error whose message
			// carries no location; net/http used to print its stack.
			var pe *lazymap.PanicError
			if errors.As(err, &pe) {
				l = l.WithField("stack", string(pe.Stack))
			}
			l.Errorf("failed to process request with url=%s", url)
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
	srv := &http.Server{
		Handler: l.Handler(mux, ""),
		// ReadTimeout:    5 * time.Minute,
		// WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 50 << 20,
	}
	return s.gs.Serve(srv, ln)
}

// Close stops accepting and lets in-flight conversions finish, up to
// WEB_SHUTDOWN_TIMEOUT. Closing only the listener let the process exit in the
// middle of every response. run() defers it last, so it runs first.
func (s *Web) Close() {
	s.gs.Close()
}
