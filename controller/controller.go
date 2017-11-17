package controller

import (
	"time"

	"github.com/niusmallnan/kube-rdns/kube"
	"github.com/niusmallnan/kube-rdns/rdns"
	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	kubeClient *kubernetes.Clientset
	rdnsClient *rdns.Client
	ingNginx   *kube.IngressNginx
	ingRes     *kube.IngressResource
}

func NewController(kubeClient *kubernetes.Clientset, rdnsClient *rdns.Client) *Controller {
	ingNginx := kube.NewIngressNginx(c.kubeClient)
	ingRes := kube.NewIngressResource(c.kubeClient)
	return &Controller{kubeClient, rdnsClient, ingNginx, ingRes}
}

func (c *Controller) refreshDomain() error {
	ips, err := c.ingNginx.ListNodeIPs()
	if err != nil {
		return err
	}

	fqdn := kube.GetRootFqdn(c.kubeClient)
	logrus.Infof("Got the fqdn: %s, and host ips: %s", fqdn, ips)

	return c.rdnsClient.ApplyDomain(fqdn, ips)
}

func (c *Controller) deployUpdated(oldObj, newObj interface{}) {
	deploy := newObj.(*apps_v1beta1.Deployment)
	if _, ok := deploy.Annotations[kube.AnnotationManagedByRDNS]; ok {
		logrus.Debugf("Controller watch deployment updated: %s", deploy.Name)
		c.refreshDomain()
	}
}

func (c *Controller) RunOnce() error {
	logrus.Info("Running once for init register")
	return c.refreshDomain()
}

func (c *Controller) RunRenewLoop() error {
	logrus.Info("Running renew loop")
	d, err := time.ParseDuration(setting.GetRenewDuration())
	if err != nil {
		return errors.Wrap(err, "Failed to parse duration")
	}
	ticker := time.NewTicker(d)
	for t := range ticker.C {
		logrus.Infof("Tick at %s", t.String())
		fqdn := kube.GetRootFqdn(c.kubeClient)
		err = c.rdnsClient.RenewDomain(fqdn)
		if err != nil {
			logrus.Errorf("Failed to renew domain: %v", err)
			continue
		}
	}
	select {}
}

func (c *Controller) WatchNginxControllerUpdate() error {
	logrus.Info("Running watch the nginx controller update")
	c.ingNginx.WatchControllerUpdate(c.deployUpdated)
	logrus.Info("Stopping watch the nginx controller update")
	return errors.New("Runtime error on WatchNginxControllerUpdate")
}

func (c *Controller) WatchIngressResource() error {
	logrus.Info("Running watch the ingress resources")
	c.ingRes.WatchEvents()
	logrus.Info("Stopping watch the nginx controller")
	return errors.New("Runtime error on WatchNginxControllerUpdate")
}
