package handlers

import (
	"net/http"

	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ClusterInfo represents basic info about a cluster
type ClusterInfo struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Version    string `json:"version"`
	NodeCount  int    `json:"nodeCount"`
	ReadyNodes int    `json:"readyNodes"`
	PodCount   int    `json:"podCount"`
}

// ListClusters returns all available clusters with basic info
func ListClusters(c *gin.Context) {
	if k8s.Manager == nil {
		c.JSON(http.StatusOK, gin.H{
			"clusters": []ClusterInfo{},
			"count":    0,
		})
		return
	}

	clusterNames := k8s.Manager.ListClusters()

	clusters := make([]ClusterInfo, 0, len(clusterNames))
	for _, name := range clusterNames {
		client, err := k8s.Manager.GetClient(name)
		if err != nil {
			continue
		}

		info := ClusterInfo{
			Name:   name,
			Status: "Connected",
		}

		// Get version
		version, err := client.Clientset.Discovery().ServerVersion()
		if err != nil {
			info.Status = "Error"
		} else {
			info.Version = version.GitVersion
		}

		// Get node count
		nodes, err := client.InformerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
		if err == nil {
			info.NodeCount = len(nodes)
			for _, node := range nodes {
				for _, cond := range node.Status.Conditions {
					if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
						info.ReadyNodes++
						break
					}
				}
			}
		}

		// Get pod count
		pods, err := client.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
		if err == nil {
			info.PodCount = len(pods)
		}

		clusters = append(clusters, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"clusters": clusters,
		"count":    len(clusters),
	})
}

// GetClusterInfo returns detailed info about a specific cluster
func GetClusterInfo(c *gin.Context) {
	if k8s.Manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "multi-cluster manager not initialized"})
		return
	}

	clusterName := c.Param("cluster")

	client, err := k8s.Manager.GetClient(clusterName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	info := gin.H{
		"name":   clusterName,
		"status": "Connected",
	}

	// Get version
	version, err := client.Clientset.Discovery().ServerVersion()
	if err == nil {
		info["version"] = version.GitVersion
		info["platform"] = version.Platform
		info["buildDate"] = version.BuildDate
	}

	// Get nodes summary
	nodes, err := client.InformerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
	if err == nil {
		readyNodes := 0
		for _, node := range nodes {
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
					readyNodes++
					break
				}
			}
		}
		info["nodeCount"] = len(nodes)
		info["readyNodes"] = readyNodes
	}

	// Get pods summary
	pods, err := client.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	if err == nil {
		running := 0
		for _, pod := range pods {
			if pod.Status.Phase == corev1.PodRunning {
				running++
			}
		}
		info["podCount"] = len(pods)
		info["runningPods"] = running
	}

	// Get namespaces count
	namespaces, err := client.InformerFactory.Core().V1().Namespaces().Lister().List(labels.Everything())
	if err == nil {
		info["namespaceCount"] = len(namespaces)
	}

	// Get deployments count
	deployments, err := client.InformerFactory.Apps().V1().Deployments().Lister().List(labels.Everything())
	if err == nil {
		info["deploymentCount"] = len(deployments)
	}

	// Get services count
	services, err := client.InformerFactory.Core().V1().Services().Lister().List(labels.Everything())
	if err == nil {
		info["serviceCount"] = len(services)
	}

	c.JSON(http.StatusOK, info)
}

// ClusterMiddleware injects the cluster client into the context
func ClusterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if k8s.Manager == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "multi-cluster manager not initialized"})
			c.Abort()
			return
		}

		clusterName := c.Param("cluster")
		if clusterName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cluster name is required"})
			c.Abort()
			return
		}

		client, err := k8s.Manager.GetClient(clusterName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Store client in context
		c.Set("clusterClient", client)
		c.Set("clusterName", clusterName)
		c.Next()
	}
}

// GetClusterClient retrieves the cluster client from the gin context
func GetClusterClient(c *gin.Context) *k8s.ClusterClient {
	client, exists := c.Get("clusterClient")
	if !exists {
		return nil
	}
	return client.(*k8s.ClusterClient)
}

// GetClusterName retrieves the cluster name from the gin context
func GetClusterName(c *gin.Context) string {
	name, exists := c.Get("clusterName")
	if !exists {
		return ""
	}
	return name.(string)
}

// ImportKubeconfigRequest represents the request to import clusters from a kubeconfig
type ImportKubeconfigRequest struct {
	KubeconfigPath string `json:"kubeconfigPath" binding:"required"`
}

// ImportClustersFromKubeconfig imports clusters from a new kubeconfig file
// POST /api/clusters/import
func ImportClustersFromKubeconfig(c *gin.Context) {
	if k8s.Manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "multi-cluster manager not initialized"})
		return
	}

	var req ImportKubeconfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kubeconfigPath is required"})
		return
	}

	added, failed, err := k8s.Manager.AddClustersFromKubeconfig(req.KubeconfigPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Import completed",
		"addedClusters":  added,
		"failedClusters": failed,
		"addedCount":     len(added),
		"failedCount":    len(failed),
	})
}

// PreviewKubeconfigRequest represents the request to preview a kubeconfig
type PreviewKubeconfigRequest struct {
	KubeconfigPath string `json:"kubeconfigPath" binding:"required"`
}

// PreviewKubeconfig returns available contexts from a kubeconfig without importing
// POST /api/kubeconfig/preview
func PreviewKubeconfig(c *gin.Context) {
	var req PreviewKubeconfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kubeconfigPath is required"})
		return
	}

	contexts, err := k8s.GetContextsFromKubeconfig(req.KubeconfigPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check which contexts are already imported
	existingMap := make(map[string]bool)
	if k8s.Manager != nil {
		existingClusters := k8s.Manager.ListClusters()
		for _, name := range existingClusters {
			existingMap[name] = true
		}
	}

	var contextInfo []gin.H
	for _, ctx := range contexts {
		contextInfo = append(contextInfo, gin.H{
			"name":     ctx,
			"imported": existingMap[ctx],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"kubeconfigPath": req.KubeconfigPath,
		"contexts":       contextInfo,
		"count":          len(contexts),
	})
}

// RemoveCluster removes a cluster from the manager
// DELETE /api/clusters/:cluster
func RemoveCluster(c *gin.Context) {
	if k8s.Manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "multi-cluster manager not initialized"})
		return
	}

	clusterName := c.Param("cluster")
	if clusterName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cluster name is required"})
		return
	}

	err := k8s.Manager.RemoveCluster(clusterName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cluster removed successfully",
		"cluster": clusterName,
	})
}

// UploadKubeconfigRequest represents the request to upload kubeconfig content
type UploadKubeconfigRequest struct {
	Content     string            `json:"content" binding:"required"`
	Filename    string            `json:"filename"`
	NameMapping map[string]string `json:"nameMapping"` // Maps original context name to custom name
}

// UploadKubeconfig handles kubeconfig file upload
// POST /api/kubeconfig/upload
func UploadKubeconfig(c *gin.Context) {
	var req UploadKubeconfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kubeconfig content is required"})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kubeconfig content is empty"})
		return
	}

	if k8s.Manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "multi-cluster manager not initialized"})
		return
	}

	contentBytes := []byte(req.Content)

	// Import clusters from the uploaded kubeconfig content with optional custom names
	added, failed, err := k8s.Manager.AddClustersFromKubeconfigContent(contentBytes, req.NameMapping)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save the kubeconfig to persistent storage
	var savedFilename string
	if len(added) > 0 {
		// Use the filename if provided, otherwise use the first cluster name
		saveName := req.Filename
		if saveName == "" {
			saveName = added[0]
		}
		savedFilename, _ = k8s.SaveKubeconfig(contentBytes, saveName)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Import completed",
		"addedClusters":  added,
		"failedClusters": failed,
		"addedCount":     len(added),
		"failedCount":    len(failed),
		"savedAs":        savedFilename,
	})
}

// PreviewUploadedKubeconfig previews contexts from uploaded kubeconfig content
// POST /api/kubeconfig/upload/preview
func PreviewUploadedKubeconfig(c *gin.Context) {
	var req UploadKubeconfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kubeconfig content is required"})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kubeconfig content is empty"})
		return
	}

	// Get contexts from the uploaded kubeconfig content
	contexts, err := k8s.GetContextsFromKubeconfigContent([]byte(req.Content))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check which contexts are already imported
	existingMap := make(map[string]bool)
	if k8s.Manager != nil {
		existingClusters := k8s.Manager.ListClusters()
		for _, name := range existingClusters {
			existingMap[name] = true
		}
	}

	var contextInfo []gin.H
	for _, ctx := range contexts {
		contextInfo = append(contextInfo, gin.H{
			"name":     ctx,
			"imported": existingMap[ctx],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"contexts": contextInfo,
		"count":    len(contexts),
	})
}

// ListStoredKubeconfigs returns the list of stored kubeconfig files
// GET /api/kubeconfigs
func ListStoredKubeconfigs(c *gin.Context) {
	configs, err := k8s.ListStoredKubeconfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"kubeconfigs":   configs,
		"count":         len(configs),
		"storageDir":    k8s.KubeconfigDir,
	})
}

// DeleteStoredKubeconfig removes a stored kubeconfig file
// DELETE /api/kubeconfigs/:name
func DeleteStoredKubeconfig(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kubeconfig name is required"})
		return
	}

	if err := k8s.DeleteStoredKubeconfig(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Kubeconfig deleted successfully",
		"name":    name,
	})
}
