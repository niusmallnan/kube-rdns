package setting

import (
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

const (
	DefaultRootDomain                      = "rancher.io"
	DefaultBaseRdnsURL                     = "https://api.rdns.rancher.io/v1"
	DefaultRnewDuration                    = "24h"
	DefaultIngressControllerResyncDuration = "5m"
)

var (
	rootDomain                      string
	baseRdnsURL                     string
	renewDuration                   string
	ingressControllerResyncDuration time.Duration
)

func Init(ctx *cli.Context) error {
	var err error
	rootDomain = ctx.String("root-domain")
	baseRdnsURL = ctx.String("base-rdns-url")
	renewDuration = ctx.String("renew-duration")
	ingressControllerResyncDuration, err = time.ParseDuration(ctx.String("ingress-controller-resync-duration"))
	if err != nil {
		return errors.Wrapf(err, "Failed to init ingress-controller-resync-duration")
	}
	return nil
}

func GetRootDomain() string {
	return rootDomain
}

func GetBaseRdnsURL() string {
	return baseRdnsURL
}

func GetRenewDuration() string {
	return renewDuration
}

func GetIngressControllerResyncDuration() time.Duration {
	return ingressControllerResyncDuration
}
