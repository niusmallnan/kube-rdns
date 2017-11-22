package kube

import (
	"github.com/niusmallnan/kube-rdns/rdns"
	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type IngressNginx struct {
	kubeClient *kubernetes.Clientset
	rdnsClient *rdns.Client
	stop       chan struct{}
}

func NewIngressNginx(kubeClient *kubernetes.Clientset, rdnsClient *rdns.Client) *IngressNginx {
	stop := make(chan struct{})
	return &IngressNginx{kubeClient, rdnsClient, stop}
}

func (n *IngressNginx) getNodePublicIP(nodeName string) (string, error) {
	node, err := n.kubeClient.CoreV1().Nodes().Get(nodeName, meta_v1.GetOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "Failed to get node %s", nodeName)
	}
	return node.Annotations[AnnotationNodePublicIP], nil
}

func (n *IngressNginx) ListNodeIPs() (ips []string, err error) {
	options := meta_v1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"app": PodNginxControllerLabel}).String()}
	pods, err := n.kubeClient.CoreV1().Pods(NamespaceIngressNginx).List(options)
	if err != nil {
		return ips, errors.Wrap(err, "Failed to list pods")
	}

	for _, p := range pods.Items {
		if p.Annotations == nil || p.Annotations[AnnotationManagedByRDNS] != "true" {
			logrus.Debugf("ListNodeIPs: skip pod /%s/%s", p.Namespace, p.Name)
			continue
		}

		nodePublicIP, err := n.getNodePublicIP(p.Spec.NodeName)
		if err != nil {
			return ips, err
		}
		if nodePublicIP != "" {
			ips = append(ips, nodePublicIP)
		} else if p.Status.HostIP != "" {
			ips = append(ips, p.Status.HostIP)
		}
	}

	return ips, nil
}

func (n *IngressNginx) WatchControllerUpdate() {
	defer close(n.stop)

	selector := fields.OneTermEqualSelector("metadata.name", DeploymentIngressNginxControllerName)
	watcher := cache.NewListWatchFromClient(n.kubeClient.AppsV1beta1().RESTClient(), "deployments", NamespaceIngressNginx, selector)
	_, wc := cache.NewInformer(watcher,
		&apps_v1beta1.Deployment{},
		setting.GetIngressControllerResyncDuration(),
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				deploy := newObj.(*apps_v1beta1.Deployment)
				if deploy.Annotations != nil && deploy.Annotations[AnnotationManagedByRDNS] == "true" {
					logrus.Debugf("Controller watch deployment updated: %s", deploy.Name)

					ips, err := n.ListNodeIPs()
					if err != nil {
						logrus.Errorf("WatchControllerUpdate: failed to get node ips: %v", err)
					}
					fqdn := GetRootFqdn(n.kubeClient)
					logrus.Infof("WatchControllerUpdate: Got the fqdn: %s, and host ips: %s", fqdn, ips)

					err = n.rdnsClient.ApplyDomain(fqdn, ips)
					if err != nil {
						logrus.Errorf("WatchControllerUpdate: failed to apply domain: %v", err)
					}
				}
			},
		})
	go wc.Run(n.stop)
	<-n.stop
}
