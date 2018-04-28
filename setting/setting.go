package setting

import (
	"time"

	"github.com/urfave/cli"
)

const (
	DefaultRootDomain            = "lb.rancher.cloud"
	DefaultBaseRdnsURL           = "http://api.rdns.rancher.cloud/v1"
	DefaultRnewDuration          = 24 * time.Hour
	DefaultIngressResyncDuration = 5 * time.Minute
)

var (
	rootDomain            string
	baseRdnsURL           string
	renewDuration         time.Duration
	ingressResyncDuration time.Duration
)

func Init(ctx *cli.Context) {
	rootDomain = ctx.String("root-domain")
	baseRdnsURL = ctx.String("base-rdns-url")
	renewDuration = ctx.Duration("renew-duration")
	ingressResyncDuration = ctx.Duration("ingress-resync-duration")
}

func GetRootDomain() string {
	return rootDomain
}

func GetBaseRdnsURL() string {
	return baseRdnsURL
}

func GetRenewDuration() time.Duration {
	return renewDuration
}

func GetIngressResyncDuration() time.Duration {
	return ingressResyncDuration
}
