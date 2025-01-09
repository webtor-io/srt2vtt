package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	app := cli.NewApp()
	app.Name = "srt2vtt"
	app.Usage = "converts srt to vtt"
	app.Version = "0.0.1"
	configure(app)
	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).Fatal("failed to serve application")
	}
}
