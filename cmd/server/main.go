package main

import (
	"log"
	"os"
	"path/filepath"

	"k8s-dashboard/pkg/api"
	"k8s-dashboard/pkg/k8s"
)

func main() {
	// Get kubeconfig path
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		home, _ := os.UserHomeDir()
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	// Set kubeconfig storage directory
	kubeconfigDir := os.Getenv("KUBECONFIG_DIR")
	if kubeconfigDir != "" {
		k8s.SetKubeconfigDir(kubeconfigDir)
	}
	log.Printf("Kubeconfig storage directory: %s", k8s.KubeconfigDir)

	// Initialize multi-cluster manager (always enabled to allow kubeconfig uploads)
	log.Println("Starting in multi-cluster mode...")

	// Try to connect to in-cluster first (when running inside Kubernetes)
	if err := k8s.InitInClusterClient(); err != nil {
		log.Printf("In-cluster config not available: %v", err)
	}

	// Also try to load from kubeconfig file (for additional clusters or local development)
	if err := k8s.InitMultiCluster(kubeconfigPath); err != nil {
		log.Printf("Warning: %v", err)
	}

	// Load any previously stored kubeconfigs
	if err := k8s.LoadStoredKubeconfigs(); err != nil {
		log.Printf("Warning: failed to load stored kubeconfigs: %v", err)
	}

	// Setup Router
	router := api.SetupRouter()

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("===========================================")
	log.Printf("  Kubernetes Dashboard")
	log.Printf("  Port: %s", port)
	log.Printf("  Mode: Multi-cluster (%d clusters)", k8s.Manager.ClusterCount())
	if k8s.Manager.ClusterCount() > 0 {
		log.Printf("  Clusters: %v", k8s.Manager.ListClusters())
	} else {
		log.Printf("  No clusters connected - upload a kubeconfig to get started")
	}
	log.Printf("===========================================")
	log.Printf("")
	log.Printf("API Endpoints:")
	log.Printf("  GET /api/clusters              - List all clusters")
	log.Printf("  GET /api/clusters/{name}/*     - Per-cluster resources")
	log.Printf("  POST /api/kubeconfig/upload    - Upload kubeconfig file")
	log.Printf("")

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
