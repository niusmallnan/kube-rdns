package main

import (
	"os"

	"github.com/niusmallnan/kube-rdns/controller"
	"github.com/niusmallnan/kube-rdns/healthcheck"
	"github.com/niusmallnan/kube-rdns/kube"
	"github.com/niusmallnan/kube-rdns/rdns"
	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Name = "kube-rdns"
	app.Version = VERSION
	app.Usage = "You need help!"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug, d",
			EnvVar: "RANCHER_DEBUG",
		},
		cli.StringFlag{
			Name:   "listen",
			Value:  ":9595",
			EnvVar: "RANCHER_SERVER_LISTEN",
		},
		cli.StringFlag{
			Name:   "root-domain",
			Value:  setting.DefaultRootDomain,
			EnvVar: "RANCHER_ROOT_DOMAIN",
		},
		cli.StringFlag{
			Name:   "base-rdns-url",
			Value:  setting.DefaultBaseRdnsURL,
			EnvVar: "RANCHER_BASE_RDNS_URL",
		},
		cli.StringFlag{
			Name:   "renew-duration",
			Value:  setting.DefaultRnewDuration,
			EnvVar: "RANCHER_RENEW_DURATION",
		},
	}
	app.Action = appMain
	app.Run(os.Args)
}

func appMain(ctx *cli.Context) error {
	if ctx.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	setting.Init(ctx)

	kubeClient, err := kube.NewClient()
	if err != nil {
		return err
	}
	rdnsClient := rdns.NewClient()
	c := controller.NewController(kubeClient, rdnsClient)

	done := make(chan error)

	go func(done chan<- error) {
		done <- healthcheck.StartHealthCheck(ctx.String("listen"))
	}(done)

	go func(done chan<- error) {
		done <- c.RunOnce()
	}(done)

	go func(done chan<- error) {
		done <- c.RunRenewLoop()
	}(done)

	c.WatchUpdate()

	err = <-done
	logrus.Errorf("Exiting kube-rdns with error: %v", err)
	return err
}
