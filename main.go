package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/niusmallnan/kube-rdns/controller"
	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
		cli.DurationFlag{
			Name:   "renew-duration",
			Value:  setting.DefaultRnewDuration,
			EnvVar: "RANCHER_RENEW_DURATION",
		},
		cli.DurationFlag{
			Name:   "ingress-resync-duration",
			Value:  setting.DefaultIngressResyncDuration,
			EnvVar: "RANCHER_INGRESS_RESYNC_DURATION",
		},
	}
	app.Action = func(ctx *cli.Context) {
		if err := appMain(ctx); err != nil {
			logrus.Errorf("Exiting kube-rdns with error: %v", err)
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

	kubeClient, err := createApiserverClient()
	if err != nil {
		handleFatalInitError(err)
	}
	c := controller.NewRDNSController(kubeClient)

	mux := http.NewServeMux()
	registerHandlers(ctx.String("listen"), c, mux)

	go handleSigterm(c, func(code int) {
		os.Exit(code)
	})

	c.Start()

	return nil
}

func createApiserverClient() (*kubernetes.Clientset, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init kube client")
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)

	return clientset, err
}

func handleFatalInitError(err error) {
	logrus.Fatalf("Error while initializing connection to Kubernetes apiserver. "+
		"This most likely means that the cluster is misconfigured (e.g., it has "+
		"invalid apiserver certificates or service accounts configuration). Reason: %s\n"+
		"Refer to the troubleshooting guide for more information: "+
		"https://github.com/kubernetes/ingress-nginx/blob/master/docs/troubleshooting.md", err)
}

type exiter func(code int)

func handleSigterm(rdnsc *controller.RDNSController, exit exiter) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	logrus.Infof("Received SIGTERM, shutting down")

	exitCode := 0
	if err := rdnsc.Stop(); err != nil {
		logrus.Infof("Error during shutdown %v", err)
		exitCode = 1
	}

	logrus.Infof("Handled quit, awaiting pod deletion")
	time.Sleep(10 * time.Second)

	logrus.Infof("Exiting with %v", exitCode)
	exit(exitCode)
}

func registerHandlers(listen string, rc *controller.RDNSController, mux *http.ServeMux) {
	// expose health check endpoint (/healthz)
	healthz.InstallHandler(mux,
		healthz.PingHealthz,
		rc,
	)

	// TODO: enable pprof

	server := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	logrus.Fatal(server.ListenAndServe())
}
