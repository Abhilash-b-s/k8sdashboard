package api

import (
	"k8s-dashboard/pkg/api/handlers"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupRouter initializes the Gin router and defines routes
func SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(corsMiddleware())

	// Serve static files
	router.Use(static.Serve("/", static.LocalFile("./static", true)))
	router.Static("/static", "./static")

	// Swagger documentation
	router.GET("/api/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API routes
	api := router.Group("/api")
	{
		// API Info endpoint
		api.GET("", handlers.GetAPIInfo)

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

		// Workloads - ReplicationControllers
		api.GET("/replicationcontrollers", handlers.GetReplicationControllers)

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
