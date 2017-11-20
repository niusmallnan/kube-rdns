package kube

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	extensions_v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type IngressResource struct {
	kubeClient *kubernetes.Clientset
	queue      *workqueue.Type
	stop       chan struct{}
}

func NewIngressResource(kubeClient *kubernetes.Clientset) *IngressResource {
	queue := workqueue.New()
	stop := make(chan struct{})
	return &IngressResource{kubeClient, queue, stop}
}

func (n *IngressResource) ignore(ing *extensions_v1beta1.Ingress) bool {
	_, ok := ing.Annotations[AnnotationHostname]
	return ok
}

func (n *IngressResource) getRdnsHostname(ing *extensions_v1beta1.Ingress) string {
	rootFqdn := GetRootFqdn(n.kubeClient)
	return fmt.Sprintf("%s.%s.%s", ing.Name, ing.Namespace, rootFqdn)
}

func (n *IngressResource) getNIPHostname(ing *extensions_v1beta1.Ingress) string {
	for _, i := range ing.Status.LoadBalancer.Ingress {
		if i.IP != "" {
			return fmt.Sprintf("%s.%s.%s.%s", ing.Name, ing.Namespace, i.IP, NIPRootDomain)
		}
	}
	logrus.Warnf("Failed to get ingress /%s/%s resource IP address", ing.Namespace, ing.Name)
	return ""
}

func (n *IngressResource) updateHostname(ing *extensions_v1beta1.Ingress) {
	if ing.Annotations == nil {
		ing.Annotations = make(map[string]string)
	}
	switch ing.Annotations[AnnotationIngressClass] {
	case "": // nginx as default
		fallthrough
	case IngressClassNginx:
		ing.Annotations[AnnotationHostname] = n.getRdnsHostname(ing)
	case IngressClassGCE:
		ing.Annotations[AnnotationHostname] = n.getNIPHostname(ing)
	}
	if _, err := n.kubeClient.ExtensionsV1beta1().Ingresses(ing.Namespace).Update(ing); err != nil {
		logrus.Errorf("Failed to update ingress resource annotations: %v", err)
	}
}

func (n *IngressResource) WatchEvents() {
	defer close(n.stop)

	watcher := cache.NewListWatchFromClient(n.kubeClient.ExtensionsV1beta1().RESTClient(), "ingresses", v1.NamespaceAll, fields.Everything())

	_, wc := cache.NewInformer(watcher,
		&extensions_v1beta1.Ingress{},
		watchIngressResourceResyncDuration,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				addIng := obj.(*extensions_v1beta1.Ingress)
				if !n.ignore(addIng) {
					logrus.Infof("Create ingress /%s/%s", addIng.Namespace, addIng.Name)
					n.queue.Add(addIng)
				}
			}})
	go wc.Run(n.stop)

	go func() {
		for {
			item, quit := n.queue.Get()
			if quit {
				return
			}
			ing := item.(*extensions_v1beta1.Ingress)
			logrus.Debugf("Ingress resource /%s/%s: begin processing", ing.Namespace, ing.Name)
			n.updateHostname(ing)
			logrus.Debugf("Ingress resource /%s/%s: done processing", ing.Namespace, ing.Name)
			n.queue.Done(item)
		}
	}()

	<-n.stop
	n.queue.ShutDown()
}
