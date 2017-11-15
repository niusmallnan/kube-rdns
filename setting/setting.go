package setting

import "github.com/urfave/cli"

const (
	DefaultRootDomain   = "rancher.io"
	DefaultBaseRdnsURL  = "https://api.rdns.rancher.io/v1"
	DefaultRnewDuration = "24h"
)

var (
	rootDomain    string
	baseRdnsURL   string
	renewDuration string
)

func Init(ctx *cli.Context) {
	rootDomain = ctx.String("root-domain")
	baseRdnsURL = ctx.String("base-rdns-url")
	renewDuration = ctx.String("renew-duration")
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
