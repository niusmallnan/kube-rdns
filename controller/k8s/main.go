package k8s

import (
	"github.com/sirupsen/logrus"
	k8scorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	secretKey = "rdns-token"
)

func GetTokenAndRootFqdn(client *kubernetes.Clientset) (string, string) {
	secret, err := client.CoreV1().Secrets(metav1.NamespaceSystem).Get(secretKey, metav1.GetOptions{})
	if err != nil {
		logrus.Warnf("Warning: failed to get token and fqdn from secret, err: %v", err)
		return "", ""
	}

	return string(secret.Data["token"]), string(secret.Data["fqdn"])
}

func SaveTokenAndRootFqdn(client *kubernetes.Clientset, token, fqdn string) error {
	_, err := client.CoreV1().Secrets(metav1.NamespaceSystem).Create(&k8scorev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretKey,
			Namespace: metav1.NamespaceSystem,
		},
		Type: k8scorev1.SecretTypeOpaque,
		StringData: map[string]string{
			"token": token,
			"fqdn":  fqdn,
		},
	})
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"token": token,
			"fqdn":  fqdn}).Fatal("Failed to save token and fqdn to secret, err: %v", err)
	}

	return err
}
