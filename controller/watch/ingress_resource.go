package watch

import (
	"fmt"

	"github.com/niusmallnan/kube-rdns/controller/k8s"
	"github.com/niusmallnan/kube-rdns/controller/rdns"
	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func NewIngressResource(kubeClient *kubernetes.Clientset, rdnsClient *rdns.Client) *IngressResource {
	queue := workqueue.New()
	stop := make(chan struct{})
	return &IngressResource{rdnsClient, kubeClient, queue, stop}
}

func (n *IngressResource) ignore(ing *extensionsv1beta1.Ingress) bool {
	if ing.Annotations == nil {
		return false
	}
	if ing.Annotations[annotationHostname] == "" {
		return false
	}
	return true
}

func (n *IngressResource) getRdnsHostname(ing *extensionsv1beta1.Ingress) string {
	_, rootFqdn := k8s.GetTokenAndRootFqdn(n.kubeClient)
	return fmt.Sprintf("%s.%s.%s", ing.Name, ing.Namespace, rootFqdn)
}

func (n *IngressResource) getIngressIps(ing *extensionsv1beta1.Ingress) []string {
	var ips []string
	for _, i := range ing.Status.LoadBalancer.Ingress {
		if i.IP != "" {
			ips = append(ips, i.IP)
		}
	}

	return ips
}

func (n *IngressResource) sync(ing *extensionsv1beta1.Ingress) {
	if ing.Annotations == nil {
		ing.Annotations = make(map[string]string)
	}
	switch ing.Annotations[annotationIngressClass] {
	case "": // nginx as default
		fallthrough
	case ingressClassNginx:
		ips := n.getIngressIps(ing)
		if err := n.rdnsClient.ApplyDomain(ips); err == nil {
			ing.Annotations[annotationHostname] = n.getRdnsHostname(ing)
		} else {
			logrus.Error(err)
		}
	default:
		logrus.Infof("Do nothing with ingress class %s", ing.Annotations[annotationIngressClass])
	}
	if _, err := n.kubeClient.ExtensionsV1beta1().Ingresses(ing.Namespace).Update(ing); err != nil {
		logrus.Errorf("Failed to update ingress resource annotations: %v", err)
	}

	// Also need to update rules for hostname when using nginx
}

func (n *IngressResource) WatchResources() {
	defer close(n.stop)

	watcher := cache.NewListWatchFromClient(n.kubeClient.ExtensionsV1beta1().RESTClient(), "ingresses", v1.NamespaceAll, fields.Everything())

	_, wc := cache.NewInformer(watcher,
		&extensionsv1beta1.Ingress{},
		setting.GetIngressResyncDuration(),
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				addIng := obj.(*extensionsv1beta1.Ingress)
				if !n.ignore(addIng) {
					logrus.Infof("Created ingress /%s/%s", addIng.Namespace, addIng.Name)
					n.queue.Add(addIng)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				newIng := newObj.(*extensionsv1beta1.Ingress)
				if !n.ignore(newIng) {
					logrus.Infof("Updated ingress /%s/%s", newIng.Namespace, newIng.Name)
					n.queue.Add(newIng)
				}
			},
		})
	go wc.Run(n.stop)

	go func() {
		for {
			item, quit := n.queue.Get()
			if quit {
				return
			}
			ing := item.(*extensionsv1beta1.Ingress)
			logrus.Debugf("Ingress resource /%s/%s: begin processing", ing.Namespace, ing.Name)
			n.sync(ing)
			logrus.Debugf("Ingress resource /%s/%s: done processing", ing.Namespace, ing.Name)
			n.queue.Done(item)
		}
	}()

	<-n.stop
	n.queue.ShutDown()
}
