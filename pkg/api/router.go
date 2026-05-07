package api

import (
	"k8s-dashboard/pkg/api/handlers"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
)

// SetupRouter initializes the Gin router and defines routes
func SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(corsMiddleware())

	// Serve static files
	router.Use(static.Serve("/", static.LocalFile("./static", true)))
	router.Static("/static", "./static")

	// Health endpoints for Kubernetes probes
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API routes
	api := router.Group("/api")
	{
		// API Info endpoint
		api.GET("", handlers.GetAPIInfo)

		// ============================================
		// MULTI-CLUSTER ENDPOINTS
		// ============================================

		// List all clusters
		api.GET("/clusters", handlers.ListClusters)

		// Import clusters from a new kubeconfig (server path)
		api.POST("/clusters/import", handlers.ImportClustersFromKubeconfig)

		// Preview contexts in a kubeconfig file (server path)
		api.POST("/kubeconfig/preview", handlers.PreviewKubeconfig)

		// Upload kubeconfig from client
		api.POST("/kubeconfig/upload", handlers.UploadKubeconfig)
		api.POST("/kubeconfig/upload/preview", handlers.PreviewUploadedKubeconfig)

		// Stored kubeconfigs management
		api.GET("/kubeconfigs", handlers.ListStoredKubeconfigs)
		api.DELETE("/kubeconfigs/:name", handlers.DeleteStoredKubeconfig)

		// Remove a cluster (must be before :cluster routes)
		api.DELETE("/clusters/:cluster", handlers.RemoveCluster)

		// Per-cluster routes (with cluster middleware)
		cluster := api.Group("/clusters/:cluster")
		cluster.Use(handlers.ClusterMiddleware())
		{
			// Cluster info
			cluster.GET("", handlers.GetClusterInfo)

			// Cluster Overview
			cluster.GET("/overview", handlers.GetClusterOverview)
			cluster.GET("/namespaces", handlers.GetClusterNamespaces)
			cluster.GET("/namespaces/:name", handlers.GetClusterNamespaceDetail)
			cluster.GET("/nodes", handlers.GetClusterNodes)
			cluster.GET("/nodes/:name", handlers.GetClusterNode)

			// Workloads - Pods
			cluster.GET("/pods", handlers.GetClusterPods)
			cluster.GET("/pods/watch", handlers.WatchClusterPods)
			cluster.GET("/pods/:namespace/:name", handlers.GetClusterPodDetail)
			cluster.PUT("/pods/:namespace/:name", handlers.UpdateClusterPod)
			cluster.DELETE("/pods/:namespace/:name", handlers.DeleteClusterPod)

			// Workloads - Deployments
			cluster.GET("/deployments", handlers.GetClusterDeployments)
			cluster.GET("/deployments/:namespace/:name", handlers.GetClusterDeploymentDetail)
			cluster.PUT("/deployments/:namespace/:name", handlers.UpdateClusterDeployment)
			cluster.DELETE("/deployments/:namespace/:name", handlers.DeleteClusterDeployment)
			cluster.PUT("/deployments/:namespace/:name/scale", handlers.ScaleClusterDeployment)
			cluster.POST("/deployments/:namespace/:name/restart", handlers.RestartClusterDeployment)

			// Workloads - DaemonSets
			cluster.GET("/daemonsets", handlers.GetClusterDaemonSets)
			cluster.GET("/daemonsets/:namespace/:name", handlers.GetClusterDaemonSetDetail)

			// Workloads - StatefulSets
			cluster.GET("/statefulsets", handlers.GetClusterStatefulSets)
			cluster.GET("/statefulsets/:namespace/:name", handlers.GetClusterStatefulSetDetail)
			cluster.PUT("/statefulsets/:namespace/:name", handlers.UpdateClusterStatefulSet)
			cluster.DELETE("/statefulsets/:namespace/:name", handlers.DeleteClusterStatefulSet)
			cluster.PUT("/statefulsets/:namespace/:name/scale", handlers.ScaleClusterStatefulSet)

			// Workloads - ReplicaSets
			cluster.GET("/replicasets", handlers.GetClusterReplicaSets)
			cluster.GET("/replicasets/:namespace/:name", handlers.GetClusterReplicaSetDetail)

			// Workloads - Jobs
			cluster.GET("/jobs", handlers.GetClusterJobs)
			cluster.GET("/jobs/:namespace/:name", handlers.GetClusterJobDetail)

			// Workloads - CronJobs
			cluster.GET("/cronjobs", handlers.GetClusterCronJobs)
			cluster.GET("/cronjobs/:namespace/:name", handlers.GetClusterCronJobDetail)

			// Service - Services
			cluster.GET("/services", handlers.GetClusterServices)
			cluster.GET("/services/:namespace/:name", handlers.GetClusterServiceDetail)

			// Service - Ingresses
			cluster.GET("/ingresses", handlers.GetClusterIngresses)
			cluster.GET("/ingresses/:namespace/:name", handlers.GetClusterIngressDetail)
			cluster.GET("/ingressclasses", handlers.GetClusterIngressClasses)

			// Service - Network Policies
			cluster.GET("/networkpolicies", handlers.GetClusterNetworkPolicies)
			cluster.GET("/networkpolicies/:namespace/:name", handlers.GetClusterNetworkPolicyDetail)

			// Config and Storage - ConfigMaps
			cluster.GET("/configmaps", handlers.GetClusterConfigMaps)
			cluster.GET("/configmaps/:namespace/:name", handlers.GetClusterConfigMapDetail)

			// Config and Storage - Secrets
			cluster.GET("/secrets", handlers.GetClusterSecrets)
			cluster.GET("/secrets/:namespace/:name", handlers.GetClusterSecretDetail)

			// Config and Storage - PVs, PVCs, StorageClasses
			cluster.GET("/persistentvolumes", handlers.GetClusterPersistentVolumes)
			cluster.GET("/persistentvolumes/:name", handlers.GetClusterPVDetail)
			cluster.GET("/persistentvolumeclaims", handlers.GetClusterPersistentVolumeClaims)
			cluster.GET("/persistentvolumeclaims/:namespace/:name", handlers.GetClusterPVCDetail)
			cluster.GET("/storageclasses", handlers.GetClusterStorageClasses)
			cluster.GET("/storageclasses/:name", handlers.GetClusterStorageClassDetail)

			// RBAC - Cluster level
			cluster.GET("/clusterroles", handlers.GetClusterClusterRoles)
			cluster.GET("/clusterroles/:name", handlers.GetClusterClusterRoleDetailView)
			cluster.GET("/clusterrolebindings", handlers.GetClusterClusterRoleBindings)
			cluster.GET("/clusterrolebindings/:name", handlers.GetClusterClusterRoleBindingDetailView)

			// RBAC - Namespace level
			cluster.GET("/roles", handlers.GetClusterRolesView)
			cluster.GET("/roles/:namespace/:name", handlers.GetClusterRoleDetailView)
			cluster.GET("/rolebindings", handlers.GetClusterRoleBindingsView)
			cluster.GET("/rolebindings/:namespace/:name", handlers.GetClusterRoleBindingDetailView)

			// Service Accounts
			cluster.GET("/serviceaccounts", handlers.GetClusterServiceAccounts)
			cluster.GET("/serviceaccounts/:namespace/:name", handlers.GetClusterServiceAccountDetail)

			// Autoscaling - HPAs
			cluster.GET("/horizontalpodautoscalers", handlers.GetClusterHPAs)
			cluster.GET("/horizontalpodautoscalers/:namespace/:name", handlers.GetClusterHPADetail)

			// Events
			cluster.GET("/events", handlers.GetClusterEvents)
			cluster.GET("/events/:namespace/:name", handlers.GetClusterEventsForResource)

			// Logs (SSE streaming)
			cluster.GET("/logs/:namespace/:pod", handlers.StreamClusterLogs)

			// Metrics
			cluster.GET("/metrics/nodes", handlers.GetClusterNodeMetrics)
			cluster.GET("/metrics/pods", handlers.GetClusterPodMetrics)

			// Generic YAML edit (kubectl-edit-style) for any supported kind
			cluster.GET("/yaml/:kind/:namespace/:name", handlers.GetClusterResourceYAML)
			cluster.PUT("/yaml/:kind/:namespace/:name", handlers.UpdateClusterResourceYAML)
		}

		// ============================================
		// LEGACY SINGLE-CLUSTER ENDPOINTS (backward compatibility)
		// These use the first available cluster
		// ============================================

		// Cluster Overview
		api.GET("/overview", handlers.GetOverview)
		api.GET("/namespaces", handlers.GetNamespaces)
		api.GET("/namespaces/:name", handlers.GetNamespaceDetail)
		api.GET("/nodes", handlers.GetNodes)
		api.GET("/nodes/:name", handlers.GetNode)

		// Workloads - Pods
		api.GET("/pods", handlers.GetPods)
		api.GET("/pods/:namespace/:name", handlers.GetPodDetail)
		api.PUT("/pods/:namespace/:name", handlers.UpdatePod)
		api.DELETE("/pods/:namespace/:name", handlers.DeletePod)

		// Workloads - Deployments
		api.GET("/deployments", handlers.GetDeployments)
		api.GET("/deployments/:namespace/:name", handlers.GetDeploymentDetail)
		api.PUT("/deployments/:namespace/:name", handlers.UpdateDeployment)
		api.DELETE("/deployments/:namespace/:name", handlers.DeleteDeployment)
		api.PUT("/deployments/:namespace/:name/scale", handlers.ScaleDeployment)
		api.POST("/deployments/:namespace/:name/restart", handlers.RestartDeployment)

		// Workloads - DaemonSets
		api.GET("/daemonsets", handlers.GetDaemonSets)
		api.GET("/daemonsets/:namespace/:name", handlers.GetDaemonSetDetail)

		// Workloads - StatefulSets
		api.GET("/statefulsets", handlers.GetStatefulSets)
		api.GET("/statefulsets/:namespace/:name", handlers.GetStatefulSetDetail)
		api.PUT("/statefulsets/:namespace/:name", handlers.UpdateStatefulSet)
		api.DELETE("/statefulsets/:namespace/:name", handlers.DeleteStatefulSet)
		api.PUT("/statefulsets/:namespace/:name/scale", handlers.ScaleStatefulSet)

		// Workloads - ReplicaSets
		api.GET("/replicasets", handlers.GetReplicaSets)
		api.GET("/replicasets/:namespace/:name", handlers.GetReplicaSetDetail)

		// Workloads - Jobs
		api.GET("/jobs", handlers.GetJobs)
		api.GET("/jobs/:namespace/:name", handlers.GetJobDetail)

		// Workloads - CronJobs
		api.GET("/cronjobs", handlers.GetCronJobs)
		api.GET("/cronjobs/:namespace/:name", handlers.GetCronJobDetail)

		// Service - Services
		api.GET("/services", handlers.GetServices)
		api.GET("/services/:namespace/:name", handlers.GetServiceDetail)

		// Service - Ingresses
		api.GET("/ingresses", handlers.GetIngresses)
		api.GET("/ingresses/:namespace/:name", handlers.GetIngressDetail)
		api.GET("/ingressclasses", handlers.GetIngressClasses)

		// Service - Network Policies
		api.GET("/networkpolicies", handlers.GetNetworkPolicies)
		api.GET("/networkpolicies/:namespace/:name", handlers.GetNetworkPolicyDetail)

		// Config and Storage - ConfigMaps
		api.GET("/configmaps", handlers.GetConfigMaps)
		api.GET("/configmaps/:namespace/:name", handlers.GetConfigMapDetail)

		// Config and Storage - Secrets
		api.GET("/secrets", handlers.GetSecrets)
		api.GET("/secrets/:namespace/:name", handlers.GetSecretDetail)

		// Config and Storage - PVs, PVCs, StorageClasses
		api.GET("/persistentvolumes", handlers.GetPersistentVolumes)
		api.GET("/persistentvolumes/:name", handlers.GetPVDetail)
		api.GET("/persistentvolumeclaims", handlers.GetPersistentVolumeClaims)
		api.GET("/persistentvolumeclaims/:namespace/:name", handlers.GetPVCDetail)
		api.GET("/storageclasses", handlers.GetStorageClasses)
		api.GET("/storageclasses/:name", handlers.GetStorageClassDetail)

		// RBAC - Cluster level
		api.GET("/clusterroles", handlers.GetClusterRoles)
		api.GET("/clusterroles/:name", handlers.GetClusterRoleDetail)
		api.GET("/clusterrolebindings", handlers.GetClusterRoleBindings)
		api.GET("/clusterrolebindings/:name", handlers.GetClusterRoleBindingDetail)

		// RBAC - Namespace level
		api.GET("/roles", handlers.GetRoles)
		api.GET("/roles/:namespace/:name", handlers.GetRoleDetail)
		api.GET("/rolebindings", handlers.GetRoleBindings)
		api.GET("/rolebindings/:namespace/:name", handlers.GetRoleBindingDetail)

		// Service Accounts
		api.GET("/serviceaccounts", handlers.GetServiceAccounts)
		api.GET("/serviceaccounts/:namespace/:name", handlers.GetServiceAccountDetail)

		// Autoscaling - HPAs
		api.GET("/horizontalpodautoscalers", handlers.GetHPAs)
		api.GET("/horizontalpodautoscalers/:namespace/:name", handlers.GetHPADetail)

		// Events
		api.GET("/events", handlers.GetEvents)
		api.GET("/events/:namespace/:name", handlers.GetEventsForResource)

		// Logs (SSE streaming)
		api.GET("/logs/:namespace/:pod", handlers.StreamLogs)

		// Metrics
		api.GET("/metrics/nodes", handlers.GetNodeMetrics)
		api.GET("/metrics/pods", handlers.GetPodMetrics)
	}

	return router
}

// CORS middleware
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
