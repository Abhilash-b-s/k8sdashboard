package k8s

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

var Clientset *kubernetes.Clientset
var MetricsClient *metricsv.Clientset
var RestConfig *rest.Config

// InitClient initializes the Kubernetes clientset
func InitClient() error {
	var err error

	// Try in-cluster config first
	RestConfig, err = rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		RestConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("failed to build config: %w", err)
		}
		log.Println("Using kubeconfig from:", kubeconfig)
	} else {
		log.Println("Using in-cluster configuration")
	}

	Clientset, err = kubernetes.NewForConfig(RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Initialize metrics client (optional - might not be available in all clusters)
	MetricsClient, err = metricsv.NewForConfig(RestConfig)
	if err != nil {
		log.Println("Warning: Metrics client not available:", err)
		MetricsClient = nil
	}

	return nil
}
