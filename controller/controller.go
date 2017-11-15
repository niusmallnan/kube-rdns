package controller

import (
	"time"

	"github.com/niusmallnan/kube-rdns/kube"
	"github.com/niusmallnan/kube-rdns/rdns"
	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Controller struct {
	kubeClient *kubernetes.Clientset
	rdnsClient *rdns.Client
}

func NewController(kubeClient *kubernetes.Clientset, rdnsClient *rdns.Client) *Controller {
	return &Controller{kubeClient, rdnsClient}
}

func (c *Controller) refreshDomain() error {
	ingNginx := kube.NewIngressNginx(c.kubeClient)
	ips, err := ingNginx.ListNodeIPs()
	if err != nil {
		return err
	}

	fqdn := kube.GetRootFqdn(c.kubeClient)
	logrus.Infof("Got the fqdn: %s, and host ips: %s", fqdn, ips)

	return c.rdnsClient.ApplyDomain(fqdn, ips)
}

func (c *Controller) podCreated(obj interface{}) {
	pod := obj.(*v1.Pod)
	logrus.Debugf("Controller watch Pod created: %s", pod.Name)
	c.refreshDomain()
}

func (c *Controller) podDeleted(obj interface{}) {
	pod := obj.(*v1.Pod)
	logrus.Debugf("Controller watch Pod delete: %s", pod.Name)
	c.refreshDomain()
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
			return err
		}
	}
	return nil
}

func (c *Controller) WatchUpdate() {
	logrus.Info("Running watch update")

	resyncPeriod := 60 * time.Second
	// create the pod watcher
	podListWatcher := cache.NewListWatchFromClient(c.kubeClient.CoreV1().RESTClient(), "pods", kube.NamespaceIngressNginx, fields.Everything())

	_, wc := cache.NewInformer(podListWatcher,
		&v1.Pod{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.podCreated,
			DeleteFunc: c.podDeleted,
		})

	go wc.Run(wait.NeverStop)
}
