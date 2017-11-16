package kube

import (
	"fmt"

	"github.com/niusmallnan/kube-rdns/setting"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	AnnotationManagedByRDNS = "rdns.rancher.io/managed"

	NamespaceIngressNginx  = "ingress-nginx"
	NamespaceSaveClusterID = "kube-system"

	DeploymentIngressNginxControllerName = "nginx-ingress-controller"

	ConfigMapClusterInfo      = "cluster-info"
	ConfigMapClusterInfoIDKey = "cluster-id"
)

func NewClient() (*kubernetes.Clientset, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init kube client")
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)

	return clientset, err
}

func GetClusterID(client *kubernetes.Clientset) string {
	options := meta_v1.GetOptions{}
	cmap, err := client.CoreV1().ConfigMaps(NamespaceSaveClusterID).Get(ConfigMapClusterInfo, options)
	if err != nil {
		logrus.Fatalf("Failed to get cluster info from configmap: %v", err)
	}

	id, ok := cmap.Data[ConfigMapClusterInfoIDKey]
	if !ok {
		logrus.Fatalf("Failed to get cluster id from configmap %s", ConfigMapClusterInfo)
	}

	logrus.Infof("Got cluster id: %s", id)
	return id
}

func GetRootFqdn(client *kubernetes.Clientset) string {
	clusterID := GetClusterID(client)
	return fmt.Sprintf("%s.%s", clusterID, setting.GetRootDomain())
}
