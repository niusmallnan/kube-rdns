package kube

import (
	"github.com/pkg/errors"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type IngressNginx struct {
	kubeClient *kubernetes.Clientset
	stop       chan struct{}
}

func NewIngressNginx(kubeClient *kubernetes.Clientset) *IngressNginx {
	stop := make(chan struct{})
	return &IngressNginx{kubeClient, stop}
}

func (n *IngressNginx) ListNodeIPs() (ips []string, err error) {
	pods, err := n.kubeClient.CoreV1().Pods(NamespaceIngressNginx).List(meta_v1.ListOptions{})
	if err != nil {
		return ips, errors.Wrap(err, "Failed to list pods")
	}

	for _, p := range pods.Items {
		if _, ok := p.Annotations[AnnotationManagedByRDNS]; ok && p.Status.HostIP != "" {
			ips = append(ips, p.Status.HostIP)
		}
	}

	return ips, nil
}

func (n *IngressNginx) WatchControllerUpdate(deployUpdated func(oldObj, newObj interface{})) {
	defer close(n.stop)

	selector := fields.OneTermEqualSelector("metadata.name", DeploymentIngressNginxControllerName)
	watcher := cache.NewListWatchFromClient(n.kubeClient.AppsV1beta1().RESTClient(), "deployments", NamespaceIngressNginx, selector)
	_, wc := cache.NewInformer(watcher,
		&apps_v1beta1.Deployment{},
		watchResyncPeriod,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: deployUpdated,
		})
	go wc.Run(n.stop)
	<-n.stop
}
