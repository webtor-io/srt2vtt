package services

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

type Serve struct {
	w  *Web
	pr *Probe
}

func NewServe(w *Web, pr *Probe) *Serve {
	return &Serve{w: w, pr: pr}
}

func (s *Serve) Serve() error {

	webError := make(chan error, 1)
	probeError := make(chan error, 1)

	go func() {
		err := s.w.Serve()
		webError <- err
	}()

	go func() {
		err := s.pr.Serve()
		probeError <- err
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigs:
		log.WithField("signal", sig).Info("Got syscall")
	case err := <-webError:
		return errors.Wrap(err, "Got Web error")
	case err := <-probeError:
		return errors.Wrap(err, "Got Probe error")
	}
	return nil
}
