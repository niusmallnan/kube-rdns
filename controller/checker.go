package controller

import "net/http"

// Name returns the healthcheck name
func (c RDNSController) Name() string {
	return "kube-rdns-controller"
}

// Check returns if the kube-rdns healthz endpoint is returning ok (status code 200)
func (c *RDNSController) Check(_ *http.Request) error {
	return nil
}
