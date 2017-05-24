package utils

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClient retrieves a kubernetes client
func GetClient() (*kubernetes.Clientset, error) {
	// make a client config with kube config
	config, err := GetK8sRestConfig()
	if err != nil {
		return nil, err
	}

	// make a client out of the kube client config
	return kubernetes.NewForConfig(config)
}

// GetK8sRestConfig returns a k8s rest client config
func GetK8sRestConfig() (conf *rest.Config, err error) {
	// retrieve necessary kube config settings
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	return kubeConfig.ClientConfig()
}
