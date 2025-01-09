package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	cs "github.com/webtor-io/common-services"
	"github.com/webtor-io/srt2vtt/services"
)

func configure(app *cli.App) {
	app.Flags = []cli.Flag{}
	app.Flags = cs.RegisterProbeFlags(app.Flags)
	app.Flags = services.RegisterWebFlags(app.Flags)
	app.Action = run
}

func run(c *cli.Context) error {
	var servers []cs.Servable
	// Setting SRT2VTTPoolService
	srt2vtt := services.NewSRT2VTT()

	// Setting Probe
	probe := cs.NewProbe(c)
	if probe != nil {
		servers = append(servers, probe)
		defer probe.Close()
	}

	// Setting Web
	web := services.NewWeb(c, srt2vtt)
	servers = append(servers, web)
	defer web.Close()

	serve := cs.NewServe(servers...)

	// And SERVE!
	err := serve.Serve()
	if err != nil {
		log.WithError(err).Error("got serve error")
	}

	return err
}
