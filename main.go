package main

import (
	"os"

	joonix "github.com/joonix/log"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	log.SetFormatter(joonix.NewFormatter())
	app := cli.NewApp()
	app.Name = "srt2vtt"
	app.Usage = "converts srt to vtt"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "host, H",
			Usage: "listening host",
			Value: "",
		},
		cli.IntFlag{
			Name:  "http-port, Ph",
			Usage: "http listening port",
			Value: 8080,
		},
		cli.StringFlag{
			Name:   "access-control-allow-origin, acao",
			Usage:  "Access-Control-Allow-Origin header value",
			EnvVar: "ACCESS_CONTROL_ALLOW_ORIGIN",
			Value:  "*",
		},
		cli.IntFlag{
			Name:  "probe-port, pP",
			Usage: "probe port",
			Value: 8081,
		},
		cli.StringFlag{
			Name:   "job-id",
			Usage:  "job id",
			Value:  "",
			EnvVar: "JOB_ID",
		},
		cli.StringFlag{
			Name:   "job-type",
			Usage:  "job type",
			Value:  "",
			EnvVar: "JOB_TYPE",
		},
	}
	app.Action = run
	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).Fatal("Failed to serve application")
	}
}
