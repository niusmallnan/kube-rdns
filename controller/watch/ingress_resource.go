package watch

import (
	"fmt"
	"strings"

	"github.com/niusmallnan/kube-rdns/controller/k8s"
	"github.com/niusmallnan/kube-rdns/controller/rdns"
	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
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
	logrus.Debugf("Got ingress resource ip addresses: %s", ips)

	return ips
}

func (n *IngressResource) sync(ing *extensionsv1beta1.Ingress) {
	fqdn := n.getRdnsHostname(ing)

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of Ingress before attempting update
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
		latestIng, err := n.kubeClient.ExtensionsV1beta1().Ingresses(ing.Namespace).Get(ing.Name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get latest version of ingress: %v", err)
			return err
		}

		if latestIng.Annotations == nil {
			latestIng.Annotations = make(map[string]string)
		}
		latestIng = latestIng.DeepCopy()
		changed := false

		switch latestIng.Annotations[annotationIngressClass] {
		case "": // nginx as default
			fallthrough
		case ingressClassNginx:
			ips := n.getIngressIps(latestIng)
			if len(ips) > 0 {
				changed = true
				if err := n.rdnsClient.ApplyDomain(ips); err == nil {
					latestIng.Annotations[annotationHostname] = fqdn
				} else {
					logrus.Error(errors.Wrap(err, "Called by ingress watch"))
					return err
				}
			}
		default:
			logrus.Infof("Do nothing with ingress class %s", latestIng.Annotations[annotationIngressClass])
		}

		if !changed {
			return nil
		}

		// Also need to update rules for hostname when using nginx
		for i, rule := range latestIng.Spec.Rules {
			logrus.Debugf("Got ingress resource hostname: %s", rule.Host)
			if strings.HasSuffix(rule.Host, setting.GetRootDomain()) {
				latestIng.Spec.Rules[i].Host = fqdn
			}
		}

		_, err = n.kubeClient.ExtensionsV1beta1().Ingresses(latestIng.Namespace).Update(latestIng)
		if err != nil {
			logrus.Errorf("Failed to update ingress resource: %v", err)
		}

		return err
	})

	if retryErr != nil {
		logrus.Errorf("Failed to retry to update ingress resource: %v", retryErr)
	}
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
