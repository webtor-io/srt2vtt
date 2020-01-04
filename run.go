package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/webtor-io/srt2vtt/services"
)

func run(c *cli.Context) error {

	// Setting ProbeService
	probe := services.NewProbe(c.String("host"), c.Int("probe-port"))
	defer probe.Close()

	// Setting SRT2VTTPoolService
	pool := services.NewSRT2VTTPool()

	// Setting WebService
	web := services.NewWeb(pool, c.String("host"), c.Int("http-port"), c.String("acao"))
	defer probe.Close()

	// Setting ServeService
	serve := services.NewServe(web, probe)

	// And SERVE!
	err := serve.Serve()
	if err != nil {
		log.WithError(err).Error("Got server error")
	}

	log.Info("Shooting down... at last!")
	return err
}
