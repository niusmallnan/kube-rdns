package controller

import "time"

const (
	DEFAULT_RENEW_DURATION = "24h"
)

// RunOnce: register domains
func RunOnce() error {
}

// RunRenewLoop: renew domains
func RunRenewLoop(duration string) error {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(d)
	for t := range ticker.C {

	}
	return nil
}

// WatchUpdate: watch the update about the ingress controllers
func WatchUpdate() error {

}
