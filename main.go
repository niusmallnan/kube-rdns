package main

import (
	"os"

	"github.com/niusmallnan/kube-rdns/controller"
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

	setting.Init(ctx)

	kubeClient, err := kube.NewClient()
	if err != nil {
		return err
	}
	rdnsClient := rdns.NewClient()
	c := controller.NewController(kubeClient, rdnsClient)

	done := make(chan error)

	go func() {
		done <- c.RunOnce()
	}()

	go func() {
		done <- c.RunRenewLoop()
	}()

	c.WatchUpdate()

	return <-done
}
