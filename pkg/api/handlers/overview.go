package handlers

import (
	"context"
	"net/http"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// checkLegacyClientAvailable checks if the single-cluster client is available
// Returns true if available, false if not (and sends error response)
func checkLegacyClientAvailable(c *gin.Context) bool {
	if k8s.InformerFactory != nil && k8s.Clientset != nil {
		return true
	}

	// Try multi-cluster mode - use first available cluster
	if k8s.Manager != nil && k8s.Manager.ClusterCount() > 0 {
		// Redirect to indicate they should use cluster-specific endpoints
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Legacy API not available in multi-cluster mode",
			"message": "Use /api/clusters/{cluster-name}/... endpoints instead",
			"clusters": k8s.Manager.ListClusters(),
		})
		return false
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "No cluster connected",
		"message": "Please upload a kubeconfig file to connect to a cluster",
	})
	return false
}

// GetAPIInfo returns API information and available endpoints
func GetAPIInfo(c *gin.Context) {
	endpoints := gin.H{
		"name":    "Kubernetes Dashboard API",
		"version": "1.0.0",
		"endpoints": gin.H{
			"overview": gin.H{
				"GET /api/overview":         "Cluster overview statistics",
				"GET /api/namespaces":       "List all namespaces",
				"GET /api/namespaces/:name": "Get namespace details",
				"GET /api/nodes":            "List all nodes",
				"GET /api/nodes/:name":      "Get node details",
			},
			"workloads": gin.H{
				"GET /api/pods":                        "List pods",
				"GET /api/pods/:namespace/:name":       "Get pod details",
				"DELETE /api/pods/:namespace/:name":    "Delete a pod",
				"GET /api/deployments":                 "List deployments",
				"GET /api/deployments/:namespace/:name": "Get deployment details",
				"GET /api/daemonsets":                  "List daemonsets",
				"GET /api/statefulsets":                "List statefulsets",
				"GET /api/replicasets":                 "List replicasets",
				"GET /api/jobs":                        "List jobs",
				"GET /api/cronjobs":                    "List cronjobs",
			},
			"networking": gin.H{
				"GET /api/services":        "List services",
				"GET /api/ingresses":       "List ingresses",
				"GET /api/ingressclasses":  "List ingress classes",
				"GET /api/networkpolicies": "List network policies",
			},
			"config": gin.H{
				"GET /api/configmaps":            "List configmaps",
				"GET /api/secrets":               "List secrets",
				"GET /api/persistentvolumes":     "List persistent volumes",
				"GET /api/persistentvolumeclaims": "List PVCs",
				"GET /api/storageclasses":        "List storage classes",
			},
			"rbac": gin.H{
				"GET /api/clusterroles":        "List cluster roles",
				"GET /api/clusterrolebindings": "List cluster role bindings",
				"GET /api/roles":               "List roles",
				"GET /api/rolebindings":        "List role bindings",
				"GET /api/serviceaccounts":     "List service accounts",
			},
			"other": gin.H{
				"GET /api/horizontalpodautoscalers": "List HPAs",
				"GET /api/events":                   "List events",
				"GET /api/logs/:namespace/:pod":     "Stream pod logs (SSE)",
				"GET /api/metrics/nodes":            "Get node metrics",
				"GET /api/metrics/pods":             "Get pod metrics",
			},
		},
		"notes": []string{
			"Most list endpoints support ?namespace= query parameter for filtering",
			"Detail endpoints are available at /:namespace/:name for namespaced resources",
		},
	}
	c.JSON(http.StatusOK, endpoints)
}

// GetOverview returns cluster overview statistics
func GetOverview(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}

	// Use Listers from the shared informer factory
	nodes, err := k8s.InformerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list nodes"})
		return
	}
	pods, err := k8s.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pods"})
		return
	}
	deploys, err := k8s.InformerFactory.Apps().V1().Deployments().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list deployments"})
		return
	}
	services, err := k8s.InformerFactory.Core().V1().Services().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list services"})
		return
	}

	// Count node status
	readyNodes := 0
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNodes++
				break
			}
		}
	}

	// Count pod status
	runningPods, pendingPods, failedPods := 0, 0, 0
	namespaceSet := make(map[string]bool)
	for _, pod := range pods {
		namespaceSet[pod.Namespace] = true
		switch pod.Status.Phase {
		case corev1.PodRunning:
			runningPods++
		case corev1.PodPending:
			pendingPods++
		case corev1.PodFailed:
			failedPods++
		}
	}

	// Get unique namespaces
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	response := OverviewResponse{
		TotalNodes:       len(nodes),
		ReadyNodes:       readyNodes,
		TotalPods:        len(pods),
		RunningPods:      runningPods,
		PendingPods:      pendingPods,
		FailedPods:       failedPods,
		TotalDeployments: len(deploys),
		TotalServices:    len(services),
		Namespaces:       namespaces,
	}

	c.JSON(http.StatusOK, response)
}

// GetNamespaces returns list of all namespaces
func GetNamespaces(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaces, err := k8s.Clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	nsList := make([]gin.H, 0, len(namespaces.Items))
	for _, ns := range namespaces.Items {
		nsList = append(nsList, gin.H{
			"name":   ns.Name,
			"status": string(ns.Status.Phase),
			"age":    FormatAge(ns.CreationTimestamp.Time),
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": nsList})
}
