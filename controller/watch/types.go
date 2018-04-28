package watch

import (
	"github.com/niusmallnan/kube-rdns/controller/rdns"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

const (
	annotationHostname     = "rdns.cattle.io/hostname"
	annotationIngressClass = "kubernetes.io/ingress.class"
	ingressClassNginx      = "nginx"
)

type IngressResource struct {
	rdnsClient *rdns.Client
	kubeClient *kubernetes.Clientset
	queue      *workqueue.Type
	stop       chan struct{}
}
