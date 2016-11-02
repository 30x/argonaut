package utils

import (
  "k8s.io/kubernetes/pkg/client/unversioned"
  "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
  "k8s.io/kubernetes/pkg/client/restclient"
)

// GetClient retrieves a kubernetes client
func GetClient() (*unversioned.Client, error) {
	// make a client config with kube config
	config, err := GetK8sRestConfig()
	if err != nil {
		return nil, err
	}

	// make a client out of the kube client config
	client, err := unversioned.New(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// GetK8sRestConfig returns a k8s rest client config
func GetK8sRestConfig() (conf *restclient.Config, err error) {
  // retrieve necessary kube config settings
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

  return kubeConfig.ClientConfig()
}