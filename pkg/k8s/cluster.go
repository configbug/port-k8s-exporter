package k8s

import (
	"context"

	"github.com/port-labs/port-k8s-exporter/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ResolveClusterName tries multiple strategies to detect the cluster name automatically.
// Strategies (in priority order):
//  1. Kubeconfig current context name – fast, no network call required.
//  2. UID of the kube-system namespace – stable across renames, works in every cluster.
//
// Returns an empty string if all strategies fail.
func ResolveClusterName(restConfig *rest.Config, kubeConfig clientcmd.ClientConfig) string {
	// Strategy 1: kubeconfig current context name
	if name := clusterNameFromKubeconfig(kubeConfig); name != "" {
		logger.Infow("Resolved cluster name from kubeconfig context", "clusterName", name)
		return name
	}

	// Strategy 2: kube-system namespace UID
	if name := clusterNameFromNamespaceUID(restConfig); name != "" {
		logger.Infow("Resolved cluster name from kube-system namespace UID", "clusterName", name)
		return name
	}

	logger.Warnw("Could not auto-detect cluster name; falling back to state key")
	return ""
}

// clusterNameFromKubeconfig reads the current kubeconfig context name.
// When running inside a cluster this will typically be empty, which is expected.
func clusterNameFromKubeconfig(kubeConfig clientcmd.ClientConfig) string {
	raw, err := kubeConfig.RawConfig()
	if err != nil {
		return ""
	}
	return raw.CurrentContext
}

// clusterNameFromNamespaceUID returns the UID of the kube-system namespace as
// a stable, unique cluster identifier. It works in-cluster and out-of-cluster.
func clusterNameFromNamespaceUID(restConfig *rest.Config) string {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logger.Debugw("Failed to create kubernetes clientset for cluster name resolution", "error", err.Error())
		return ""
	}

	ns, err := clientset.CoreV1().Namespaces().Get(context.Background(), "kube-system", metav1.GetOptions{})
	if err != nil {
		logger.Debugw("Failed to get kube-system namespace for cluster name resolution", "error", err.Error())
		return ""
	}

	return string(ns.UID)
}
