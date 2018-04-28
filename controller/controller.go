package controller

import (
	"time"

	"github.com/niusmallnan/kube-rdns/controller/rdns"
	"github.com/niusmallnan/kube-rdns/controller/watch"
	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	podNginxControllerLabel      = "ingress-nginx"
	defaultNginxIngressNamespace = "ingress-nginx"
	rkeInternalAddressAnnotation = "rke.cattle.io/internal-ip"
	rkeExternalAddressAnnotation = "rke.cattle.io/external-ip"
)

type RDNSController struct {
	rdnsClient *rdns.Client
	kubeClient *kubernetes.Clientset
	ingRes     *watch.IngressResource
}

func NewRDNSController(kubeClient *kubernetes.Clientset) *RDNSController {
	rdnsClient := rdns.NewClient(kubeClient)
	ingRes := watch.NewIngressResource(kubeClient, rdnsClient)
	return &RDNSController{
		rdnsClient: rdnsClient,
		ingRes:     ingRes,
	}
}

func (c *RDNSController) Stop() error {
	return nil
}

func (c *RDNSController) Start() {
	ips, err := c.getNginxControllerIPs()
	if err != nil {
		logrus.Fatalf("Fail to get nginx controller ips on start(), err: %s", err)
	}

	logrus.Infof("Got the host ips: %s", ips)
	if err = c.rdnsClient.ApplyDomain(ips); err != nil {
		logrus.Error(err)
	}

	logrus.Info("Running watch the ingress resources")
	go c.ingRes.WatchResources()

	c.renewLoop()
	select {}
}

func (c *RDNSController) renewLoop() {
	logrus.Info("Running renew loop")
	ticker := time.NewTicker(setting.GetRenewDuration())
	for t := range ticker.C {
		logrus.Infof("Tick at %s", t.String())
		if err := c.rdnsClient.RenewDomain(); err != nil {
			logrus.Errorf("Failed to renew domain: %v", err)
			continue
		}
	}
}

func (c *RDNSController) getNginxControllerIPs() ([]string, error) {
	var ips []string

	options := metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"app": podNginxControllerLabel}).String()}
	pods, err := c.kubeClient.CoreV1().Pods(defaultNginxIngressNamespace).List(options)

	if err != nil {
		logrus.WithError(err).Error("syncing ingress rules to rdns server error")
		return nil, err
	}
	for _, pod := range pods.Items {
		ip, err := c.getNodePublicIP(pod.Spec.NodeName)
		if err != nil {
			logrus.WithError(err).Errorf("get node %s public ip error", pod.Spec.NodeName)
			continue
		}
		ips = append(ips, ip)
	}
	return ips, nil
}

func (c *RDNSController) getNodePublicIP(nodeName string) (string, error) {
	node, err := c.kubeClient.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var ip string
	for _, address := range node.Status.Addresses {
		if address.Type == "ExternalIP" {
			return address.Address, nil
		}
		if address.Type == "InternalIP" {
			ip = address.Address
		}
	}

	//from annotation
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	if ip, ok := node.Annotations[rkeExternalAddressAnnotation]; ok {
		return ip, nil
	}

	if ip, ok := node.Annotations[rkeInternalAddressAnnotation]; ok {
		return ip, nil
	}

	return ip, nil
}
