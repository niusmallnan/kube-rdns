package kube

import (
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

func (n *IngressResource) updateHostname(ing *extensions_v1beta1.Ingress) {

}

func (n *IngressResource) WatchEvents() {
	defer close(n.stop)

	watcher := cache.NewListWatchFromClient(n.kubeClient.ExtensionsV1beta1().RESTClient(), "ingresses", v1.NamespaceAll, fields.Everything())

	_, wc := cache.NewInformer(watcher,
		&extensions_v1beta1.Ingress{},
		watchResyncPeriod,
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
			logrus.Debugf("Ingress resource: begin processing %v", item)
			n.updateHostname(item.(*extensions_v1beta1.Ingress))
			logrus.Debugf("Ingress resource: done processing %v", item)
			n.queue.Done(item)
		}
	}()

	<-n.stop
	n.queue.ShutDown()
}
