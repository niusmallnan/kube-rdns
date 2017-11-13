package main

import (
	"os"

	"github.com/niusmallnan/kube-rdns/controller"
	"github.com/niusmallnan/kube-rdns/rdns"
	"github.com/niusmallnan/kube-rdns/source"
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
			Name:   "base-url",
			Value:  rdns.BaseRequestURL,
			EnvVar: "RANCHER_BASE_URL",
		},
		cli.StringFlag{
			Name:   "ingress-nginx-namespace",
			Value:  source.INGRESS_NGINX_NS,
			EnvVar: "RANCHER_INGRESS_NGINX_NS",
		},
		cli.StringFlag{
			Name:   "renew-duration",
			Value:  controller.DEFAULT_RENEW_DURATION,
			EnvVar: "RANCHER_RENEW_DURATIONL",
		},
	}
	app.Action = func(ctx *cli.Context) {
		if err := appMain(ctx); err != nil {
			logrus.Errorf("error: %v", err)
			os.Exit(1)
		}
	}

	app.Run(os.Args)
}

func appMain(ctx *cli.Context) error {
	if ctx.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	rdnsClient := rdns.NewClient(ctx.String("base-url"))

	done := make(chan error)

	go func() {
		done <- controller.RunOnce()
	}()

	go func() {
		done <- controller.RunRenewLoop(ctx.String("renew-duration"))
	}()

	go func() {
		done <- controller.WatchUpdate()
	}()

	return <-done
}
