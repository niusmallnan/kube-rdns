package kube

import (
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type IngressNginx struct {
	kubeClient *kubernetes.Clientset
}

func NewIngressNginx(kubeClient *kubernetes.Clientset) *IngressNginx {
	return &IngressNginx{kubeClient}
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
