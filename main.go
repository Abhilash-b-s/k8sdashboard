package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	clientset       *kubernetes.Clientset
	metricsClient   *metricsv.Clientset
	informerFactory informers.SharedInformerFactory
	stopCh          = make(chan struct{})
	cacheMu         sync.RWMutex

	// Cached resources
	cachedNodes                  []corev1.Node
	cachedPods                   []corev1.Pod
	cachedDeployments            []appsv1.Deployment
	cachedDaemonSets             []appsv1.DaemonSet
	cachedStatefulSets           []appsv1.StatefulSet
	cachedReplicaSets            []appsv1.ReplicaSet
	cachedJobs                   []batchv1.Job
	cachedCronJobs               []batchv1.CronJob
	cachedServices               []corev1.Service
	cachedIngresses              []networkingv1.Ingress
	cachedConfigMaps             []corev1.ConfigMap
	cachedSecrets                []corev1.Secret
	cachedPVCs                   []corev1.PersistentVolumeClaim
	cachedPVs                    []corev1.PersistentVolume
	cachedStorageClasses         []storagev1.StorageClass
	cachedNamespaces             []corev1.Namespace
	cachedClusterRoles           []rbacv1.ClusterRole
	cachedClusterRoleBindings    []rbacv1.ClusterRoleBinding
	cachedServiceAccounts        []corev1.ServiceAccount
	cachedReplicationControllers []corev1.ReplicationController
	cachedEndpoints              []corev1.Endpoints
	cachedIngressClasses         []networkingv1.IngressClass
)

func main() {
	if err := initK8sClient(); err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	startInformers()
	log.Println("Waiting for informer caches to sync...")
	time.Sleep(2 * time.Second)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(corsMiddleware())

	router.StaticFile("/", "./static/index.html")
	router.Static("/static", "./static")

	api := router.Group("/api")
	{
		// Overview
		api.GET("/overview", getOverview)
		api.GET("/namespaces", getNamespaces)

		// Cluster
		api.GET("/nodes", getNodes)
		api.GET("/nodes/:name", getNodeDetail)
		api.GET("/persistentvolumes", getPersistentVolumes)
		api.GET("/storageclasses", getStorageClasses)
		api.GET("/clusterroles", getClusterRoles)
		api.GET("/clusterrolebindings", getClusterRoleBindings)

		// Workloads
		api.GET("/pods", getPods)
		api.GET("/pods/:namespace/:name", getPodDetail)
		api.DELETE("/pods/:namespace/:name", deletePod)
		api.GET("/deployments", getDeployments)
		api.GET("/deployments/:namespace/:name", getDeploymentDetail)
		api.GET("/daemonsets", getDaemonSets)
		api.GET("/statefulsets", getStatefulSets)
		api.GET("/replicasets", getReplicaSets)
		api.GET("/replicationcontrollers", getReplicationControllers)
		api.GET("/jobs", getJobs)
		api.GET("/cronjobs", getCronJobs)

		// Service
		api.GET("/services", getServices)
		api.GET("/services/:namespace/:name", getServiceDetail)
		api.GET("/ingresses", getIngresses)
		api.GET("/ingressclasses", getIngressClasses)
		api.GET("/endpoints", getEndpoints)

		// Config and Storage
		api.GET("/configmaps", getConfigMaps)
		api.GET("/configmaps/:namespace/:name", getConfigMapDetail)
		api.GET("/secrets", getSecrets)
		api.GET("/secrets/:namespace/:name", getSecretDetail)
		api.GET("/persistentvolumeclaims", getPersistentVolumeClaims)
		api.GET("/serviceaccounts", getServiceAccounts)

		// Logs
		api.GET("/logs/:namespace/:pod", streamLogs)

		// Metrics
		api.GET("/metrics/nodes", getNodeMetrics)
		api.GET("/metrics/pods", getPodMetrics)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting Kubernetes Dashboard on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initK8sClient() error {
	var config *rest.Config
	var err error

	config, err = rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("failed to build config: %w", err)
		}
		log.Println("Using kubeconfig from:", kubeconfig)
	} else {
		log.Println("Using in-cluster configuration")
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	metricsClient, err = metricsv.NewForConfig(config)
	if err != nil {
		log.Printf("Warning: metrics client not available: %v", err)
	}

	return nil
}

func startInformers() {
	informerFactory = informers.NewSharedInformerFactory(clientset, 30*time.Second)

	// Core resources
	informerFactory.Core().V1().Nodes().Informer().AddEventHandler(makeHandler(updateNodeCache))
	informerFactory.Core().V1().Pods().Informer().AddEventHandler(makeHandler(updatePodCache))
	informerFactory.Core().V1().Services().Informer().AddEventHandler(makeHandler(updateServiceCache))
	informerFactory.Core().V1().ConfigMaps().Informer().AddEventHandler(makeHandler(updateConfigMapCache))
	informerFactory.Core().V1().Secrets().Informer().AddEventHandler(makeHandler(updateSecretCache))
	informerFactory.Core().V1().PersistentVolumeClaims().Informer().AddEventHandler(makeHandler(updatePVCCache))
	informerFactory.Core().V1().PersistentVolumes().Informer().AddEventHandler(makeHandler(updatePVCache))
	informerFactory.Core().V1().Namespaces().Informer().AddEventHandler(makeHandler(updateNamespaceCache))
	informerFactory.Core().V1().ServiceAccounts().Informer().AddEventHandler(makeHandler(updateServiceAccountCache))
	informerFactory.Core().V1().ReplicationControllers().Informer().AddEventHandler(makeHandler(updateReplicationControllerCache))
	informerFactory.Core().V1().Endpoints().Informer().AddEventHandler(makeHandler(updateEndpointsCache))

	// Apps resources
	informerFactory.Apps().V1().Deployments().Informer().AddEventHandler(makeHandler(updateDeploymentCache))
	informerFactory.Apps().V1().DaemonSets().Informer().AddEventHandler(makeHandler(updateDaemonSetCache))
	informerFactory.Apps().V1().StatefulSets().Informer().AddEventHandler(makeHandler(updateStatefulSetCache))
	informerFactory.Apps().V1().ReplicaSets().Informer().AddEventHandler(makeHandler(updateReplicaSetCache))

	// Batch resources
	informerFactory.Batch().V1().Jobs().Informer().AddEventHandler(makeHandler(updateJobCache))
	informerFactory.Batch().V1().CronJobs().Informer().AddEventHandler(makeHandler(updateCronJobCache))

	// Networking resources
	informerFactory.Networking().V1().Ingresses().Informer().AddEventHandler(makeHandler(updateIngressCache))
	informerFactory.Networking().V1().IngressClasses().Informer().AddEventHandler(makeHandler(updateIngressClassCache))

	// Storage resources
	informerFactory.Storage().V1().StorageClasses().Informer().AddEventHandler(makeHandler(updateStorageClassCache))

	// RBAC resources
	informerFactory.Rbac().V1().ClusterRoles().Informer().AddEventHandler(makeHandler(updateClusterRoleCache))
	informerFactory.Rbac().V1().ClusterRoleBindings().Informer().AddEventHandler(makeHandler(updateClusterRoleBindingCache))

	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)
	log.Println("Informer caches synced successfully")
}

type handlerFunc func()

func makeHandler(fn handlerFunc) *handlerFuncs {
	return &handlerFuncs{fn: fn}
}

type handlerFuncs struct {
	fn handlerFunc
}

func (h *handlerFuncs) OnAdd(obj interface{}, isInInitialList bool) { h.fn() }
func (h *handlerFuncs) OnUpdate(oldObj, newObj interface{})         { h.fn() }
func (h *handlerFuncs) OnDelete(obj interface{})                    { h.fn() }

// Cache update functions
func updateNodeCache() {
	nodes, _ := informerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedNodes = make([]corev1.Node, 0, len(nodes))
	for _, n := range nodes {
		cachedNodes = append(cachedNodes, *n)
	}
	cacheMu.Unlock()
}

func updatePodCache() {
	pods, _ := informerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedPods = make([]corev1.Pod, 0, len(pods))
	for _, p := range pods {
		cachedPods = append(cachedPods, *p)
	}
	cacheMu.Unlock()
}

func updateDeploymentCache() {
	deploys, _ := informerFactory.Apps().V1().Deployments().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedDeployments = make([]appsv1.Deployment, 0, len(deploys))
	for _, d := range deploys {
		cachedDeployments = append(cachedDeployments, *d)
	}
	cacheMu.Unlock()
}

func updateDaemonSetCache() {
	items, _ := informerFactory.Apps().V1().DaemonSets().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedDaemonSets = make([]appsv1.DaemonSet, 0, len(items))
	for _, i := range items {
		cachedDaemonSets = append(cachedDaemonSets, *i)
	}
	cacheMu.Unlock()
}

func updateStatefulSetCache() {
	items, _ := informerFactory.Apps().V1().StatefulSets().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedStatefulSets = make([]appsv1.StatefulSet, 0, len(items))
	for _, i := range items {
		cachedStatefulSets = append(cachedStatefulSets, *i)
	}
	cacheMu.Unlock()
}

func updateReplicaSetCache() {
	items, _ := informerFactory.Apps().V1().ReplicaSets().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedReplicaSets = make([]appsv1.ReplicaSet, 0, len(items))
	for _, i := range items {
		cachedReplicaSets = append(cachedReplicaSets, *i)
	}
	cacheMu.Unlock()
}

func updateJobCache() {
	items, _ := informerFactory.Batch().V1().Jobs().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedJobs = make([]batchv1.Job, 0, len(items))
	for _, i := range items {
		cachedJobs = append(cachedJobs, *i)
	}
	cacheMu.Unlock()
}

func updateCronJobCache() {
	items, _ := informerFactory.Batch().V1().CronJobs().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedCronJobs = make([]batchv1.CronJob, 0, len(items))
	for _, i := range items {
		cachedCronJobs = append(cachedCronJobs, *i)
	}
	cacheMu.Unlock()
}

func updateServiceCache() {
	items, _ := informerFactory.Core().V1().Services().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedServices = make([]corev1.Service, 0, len(items))
	for _, i := range items {
		cachedServices = append(cachedServices, *i)
	}
	cacheMu.Unlock()
}

func updateIngressCache() {
	items, _ := informerFactory.Networking().V1().Ingresses().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedIngresses = make([]networkingv1.Ingress, 0, len(items))
	for _, i := range items {
		cachedIngresses = append(cachedIngresses, *i)
	}
	cacheMu.Unlock()
}

func updateIngressClassCache() {
	items, _ := informerFactory.Networking().V1().IngressClasses().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedIngressClasses = make([]networkingv1.IngressClass, 0, len(items))
	for _, i := range items {
		cachedIngressClasses = append(cachedIngressClasses, *i)
	}
	cacheMu.Unlock()
}

func updateConfigMapCache() {
	items, _ := informerFactory.Core().V1().ConfigMaps().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedConfigMaps = make([]corev1.ConfigMap, 0, len(items))
	for _, i := range items {
		cachedConfigMaps = append(cachedConfigMaps, *i)
	}
	cacheMu.Unlock()
}

func updateSecretCache() {
	items, _ := informerFactory.Core().V1().Secrets().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedSecrets = make([]corev1.Secret, 0, len(items))
	for _, i := range items {
		cachedSecrets = append(cachedSecrets, *i)
	}
	cacheMu.Unlock()
}

func updatePVCCache() {
	items, _ := informerFactory.Core().V1().PersistentVolumeClaims().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedPVCs = make([]corev1.PersistentVolumeClaim, 0, len(items))
	for _, i := range items {
		cachedPVCs = append(cachedPVCs, *i)
	}
	cacheMu.Unlock()
}

func updatePVCache() {
	items, _ := informerFactory.Core().V1().PersistentVolumes().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedPVs = make([]corev1.PersistentVolume, 0, len(items))
	for _, i := range items {
		cachedPVs = append(cachedPVs, *i)
	}
	cacheMu.Unlock()
}

func updateStorageClassCache() {
	items, _ := informerFactory.Storage().V1().StorageClasses().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedStorageClasses = make([]storagev1.StorageClass, 0, len(items))
	for _, i := range items {
		cachedStorageClasses = append(cachedStorageClasses, *i)
	}
	cacheMu.Unlock()
}

func updateNamespaceCache() {
	items, _ := informerFactory.Core().V1().Namespaces().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedNamespaces = make([]corev1.Namespace, 0, len(items))
	for _, i := range items {
		cachedNamespaces = append(cachedNamespaces, *i)
	}
	cacheMu.Unlock()
}

func updateClusterRoleCache() {
	items, _ := informerFactory.Rbac().V1().ClusterRoles().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedClusterRoles = make([]rbacv1.ClusterRole, 0, len(items))
	for _, i := range items {
		cachedClusterRoles = append(cachedClusterRoles, *i)
	}
	cacheMu.Unlock()
}

func updateClusterRoleBindingCache() {
	items, _ := informerFactory.Rbac().V1().ClusterRoleBindings().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedClusterRoleBindings = make([]rbacv1.ClusterRoleBinding, 0, len(items))
	for _, i := range items {
		cachedClusterRoleBindings = append(cachedClusterRoleBindings, *i)
	}
	cacheMu.Unlock()
}

func updateServiceAccountCache() {
	items, _ := informerFactory.Core().V1().ServiceAccounts().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedServiceAccounts = make([]corev1.ServiceAccount, 0, len(items))
	for _, i := range items {
		cachedServiceAccounts = append(cachedServiceAccounts, *i)
	}
	cacheMu.Unlock()
}

func updateReplicationControllerCache() {
	items, _ := informerFactory.Core().V1().ReplicationControllers().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedReplicationControllers = make([]corev1.ReplicationController, 0, len(items))
	for _, i := range items {
		cachedReplicationControllers = append(cachedReplicationControllers, *i)
	}
	cacheMu.Unlock()
}

func updateEndpointsCache() {
	items, _ := informerFactory.Core().V1().Endpoints().Lister().List(labels.Everything())
	cacheMu.Lock()
	cachedEndpoints = make([]corev1.Endpoints, 0, len(items))
	for _, i := range items {
		cachedEndpoints = append(cachedEndpoints, *i)
	}
	cacheMu.Unlock()
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// Helper functions
func formatAge(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 24*365 {
		return fmt.Sprintf("%dy", int(duration.Hours()/(24*365)))
	}
	if duration.Hours() >= 24 {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
	if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	if duration.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}

func filterByNamespace(namespace string, itemNs string) bool {
	return namespace == "" || namespace == itemNs
}

func getLabelsMap(labels map[string]string) map[string]string {
	if labels == nil {
		return make(map[string]string)
	}
	return labels
}

// API Handlers

func getOverview(c *gin.Context) {
	cacheMu.RLock()
	nodes := cachedNodes
	pods := cachedPods
	deploys := cachedDeployments
	daemonsets := cachedDaemonSets
	statefulsets := cachedStatefulSets
	services := cachedServices
	namespaces := cachedNamespaces
	cacheMu.RUnlock()

	readyNodes := 0
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNodes++
				break
			}
		}
	}

	runningPods, pendingPods, failedPods, succeededPods := 0, 0, 0, 0
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			runningPods++
		case corev1.PodPending:
			pendingPods++
		case corev1.PodFailed:
			failedPods++
		case corev1.PodSucceeded:
			succeededPods++
		}
	}

	nsList := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		nsList = append(nsList, ns.Name)
	}
	sort.Strings(nsList)

	c.JSON(http.StatusOK, gin.H{
		"totalNodes":       len(nodes),
		"readyNodes":       readyNodes,
		"totalPods":        len(pods),
		"runningPods":      runningPods,
		"pendingPods":      pendingPods,
		"failedPods":       failedPods,
		"succeededPods":    succeededPods,
		"totalDeployments": len(deploys),
		"totalDaemonSets":  len(daemonsets),
		"totalStatefulSets": len(statefulsets),
		"totalServices":    len(services),
		"namespaces":       nsList,
	})
}

func getNamespaces(c *gin.Context) {
	cacheMu.RLock()
	namespaces := cachedNamespaces
	cacheMu.RUnlock()

	result := make([]gin.H, 0, len(namespaces))
	for _, ns := range namespaces {
		result = append(result, gin.H{
			"name":   ns.Name,
			"status": string(ns.Status.Phase),
			"age":    formatAge(ns.CreationTimestamp.Time),
			"labels": getLabelsMap(ns.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getNodes(c *gin.Context) {
	cacheMu.RLock()
	nodes := cachedNodes
	cacheMu.RUnlock()

	result := make([]gin.H, 0, len(nodes))
	for _, node := range nodes {
		ready := "Unknown"
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionTrue {
					ready = "Ready"
				} else {
					ready = "NotReady"
				}
				break
			}
		}

		cpu := node.Status.Capacity.Cpu().String()
		memory := node.Status.Capacity.Memory().String()

		result = append(result, gin.H{
			"name":              node.Name,
			"status":            ready,
			"roles":             getNodeRoles(node),
			"age":               formatAge(node.CreationTimestamp.Time),
			"version":           node.Status.NodeInfo.KubeletVersion,
			"internalIP":        getNodeInternalIP(node),
			"osImage":           node.Status.NodeInfo.OSImage,
			"containerRuntime":  node.Status.NodeInfo.ContainerRuntimeVersion,
			"cpu":               cpu,
			"memory":            memory,
			"labels":            getLabelsMap(node.Labels),
			"conditions":        getNodeConditions(node),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getNodeRoles(node corev1.Node) []string {
	roles := []string{}
	for label := range node.Labels {
		if len(label) > 24 && label[:24] == "node-role.kubernetes.io/" {
			roles = append(roles, label[24:])
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "<none>")
	}
	return roles
}

func getNodeInternalIP(node corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}

func getNodeConditions(node corev1.Node) []gin.H {
	conditions := make([]gin.H, 0, len(node.Status.Conditions))
	for _, c := range node.Status.Conditions {
		conditions = append(conditions, gin.H{
			"type":               string(c.Type),
			"status":             string(c.Status),
			"lastProbeTime":      c.LastHeartbeatTime.Time,
			"lastTransitionTime": c.LastTransitionTime.Time,
			"reason":             c.Reason,
			"message":            c.Message,
		})
	}
	return conditions
}

func getNodeDetail(c *gin.Context) {
	name := c.Param("name")
	cacheMu.RLock()
	nodes := cachedNodes
	pods := cachedPods
	cacheMu.RUnlock()

	var node *corev1.Node
	for i := range nodes {
		if nodes[i].Name == name {
			node = &nodes[i]
			break
		}
	}

	if node == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	nodePods := []gin.H{}
	for _, pod := range pods {
		if pod.Spec.NodeName == name {
			nodePods = append(nodePods, gin.H{
				"name":      pod.Name,
				"namespace": pod.Namespace,
				"status":    string(pod.Status.Phase),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name":             node.Name,
		"labels":           getLabelsMap(node.Labels),
		"annotations":      getLabelsMap(node.Annotations),
		"creationTime":     node.CreationTimestamp.Time,
		"conditions":       getNodeConditions(*node),
		"capacity":         node.Status.Capacity,
		"allocatable":      node.Status.Allocatable,
		"nodeInfo":         node.Status.NodeInfo,
		"pods":             nodePods,
	})
}

func getPods(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	pods := cachedPods
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, pod := range pods {
		if !filterByNamespace(namespace, pod.Namespace) {
			continue
		}

		readyContainers := 0
		totalContainers := len(pod.Spec.Containers)
		var restarts int32 = 0
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyContainers++
			}
			restarts += cs.RestartCount
		}

		images := make([]string, 0, len(pod.Spec.Containers))
		for _, container := range pod.Spec.Containers {
			images = append(images, container.Image)
		}

		result = append(result, gin.H{
			"name":       pod.Name,
			"namespace":  pod.Namespace,
			"status":     string(pod.Status.Phase),
			"ready":      fmt.Sprintf("%d/%d", readyContainers, totalContainers),
			"restarts":   restarts,
			"age":        formatAge(pod.CreationTimestamp.Time),
			"node":       pod.Spec.NodeName,
			"ip":         pod.Status.PodIP,
			"images":     images,
			"labels":     getLabelsMap(pod.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getPodDetail(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	cacheMu.RLock()
	pods := cachedPods
	cacheMu.RUnlock()

	var pod *corev1.Pod
	for i := range pods {
		if pods[i].Namespace == namespace && pods[i].Name == name {
			pod = &pods[i]
			break
		}
	}

	if pod == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	containers := make([]gin.H, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		var status gin.H
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == container.Name {
				status = gin.H{
					"ready":        cs.Ready,
					"restartCount": cs.RestartCount,
					"started":      cs.Started,
				}
				break
			}
		}
		containers = append(containers, gin.H{
			"name":      container.Name,
			"image":     container.Image,
			"ports":     container.Ports,
			"resources": container.Resources,
			"status":    status,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"name":         pod.Name,
		"namespace":    pod.Namespace,
		"labels":       getLabelsMap(pod.Labels),
		"annotations":  getLabelsMap(pod.Annotations),
		"status":       string(pod.Status.Phase),
		"node":         pod.Spec.NodeName,
		"ip":           pod.Status.PodIP,
		"hostIP":       pod.Status.HostIP,
		"creationTime": pod.CreationTimestamp.Time,
		"containers":   containers,
		"conditions":   pod.Status.Conditions,
	})
}

func deletePod(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	err := clientset.CoreV1().Pods(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deleted successfully"})
}

func getDeployments(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	deploys := cachedDeployments
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, d := range deploys {
		if !filterByNamespace(namespace, d.Namespace) {
			continue
		}
		replicas := int32(0)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		result = append(result, gin.H{
			"name":      d.Name,
			"namespace": d.Namespace,
			"ready":     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, replicas),
			"upToDate":  d.Status.UpdatedReplicas,
			"available": d.Status.AvailableReplicas,
			"age":       formatAge(d.CreationTimestamp.Time),
			"images":    getDeploymentImages(d),
			"labels":    getLabelsMap(d.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getDeploymentImages(d appsv1.Deployment) []string {
	images := make([]string, 0)
	for _, container := range d.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}
	return images
}

func getDeploymentDetail(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	cacheMu.RLock()
	deploys := cachedDeployments
	cacheMu.RUnlock()

	var deploy *appsv1.Deployment
	for i := range deploys {
		if deploys[i].Namespace == namespace && deploys[i].Name == name {
			deploy = &deploys[i]
			break
		}
	}

	if deploy == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        deploy.Name,
		"namespace":   deploy.Namespace,
		"labels":      getLabelsMap(deploy.Labels),
		"annotations": getLabelsMap(deploy.Annotations),
		"replicas":    deploy.Spec.Replicas,
		"selector":    deploy.Spec.Selector,
		"strategy":    deploy.Spec.Strategy,
		"status":      deploy.Status,
		"conditions":  deploy.Status.Conditions,
	})
}

func getDaemonSets(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedDaemonSets
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, d := range items {
		if !filterByNamespace(namespace, d.Namespace) {
			continue
		}
		result = append(result, gin.H{
			"name":             d.Name,
			"namespace":        d.Namespace,
			"desired":          d.Status.DesiredNumberScheduled,
			"current":          d.Status.CurrentNumberScheduled,
			"ready":            d.Status.NumberReady,
			"upToDate":         d.Status.UpdatedNumberScheduled,
			"available":        d.Status.NumberAvailable,
			"age":              formatAge(d.CreationTimestamp.Time),
			"labels":           getLabelsMap(d.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getStatefulSets(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedStatefulSets
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, s := range items {
		if !filterByNamespace(namespace, s.Namespace) {
			continue
		}
		replicas := int32(0)
		if s.Spec.Replicas != nil {
			replicas = *s.Spec.Replicas
		}
		result = append(result, gin.H{
			"name":      s.Name,
			"namespace": s.Namespace,
			"ready":     fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, replicas),
			"age":       formatAge(s.CreationTimestamp.Time),
			"labels":    getLabelsMap(s.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getReplicaSets(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedReplicaSets
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, r := range items {
		if !filterByNamespace(namespace, r.Namespace) {
			continue
		}
		replicas := int32(0)
		if r.Spec.Replicas != nil {
			replicas = *r.Spec.Replicas
		}
		result = append(result, gin.H{
			"name":      r.Name,
			"namespace": r.Namespace,
			"desired":   replicas,
			"current":   r.Status.Replicas,
			"ready":     r.Status.ReadyReplicas,
			"age":       formatAge(r.CreationTimestamp.Time),
			"labels":    getLabelsMap(r.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getReplicationControllers(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedReplicationControllers
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, r := range items {
		if !filterByNamespace(namespace, r.Namespace) {
			continue
		}
		replicas := int32(0)
		if r.Spec.Replicas != nil {
			replicas = *r.Spec.Replicas
		}
		result = append(result, gin.H{
			"name":      r.Name,
			"namespace": r.Namespace,
			"desired":   replicas,
			"current":   r.Status.Replicas,
			"ready":     r.Status.ReadyReplicas,
			"age":       formatAge(r.CreationTimestamp.Time),
			"labels":    getLabelsMap(r.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getJobs(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedJobs
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, j := range items {
		if !filterByNamespace(namespace, j.Namespace) {
			continue
		}
		completions := int32(1)
		if j.Spec.Completions != nil {
			completions = *j.Spec.Completions
		}
		result = append(result, gin.H{
			"name":        j.Name,
			"namespace":   j.Namespace,
			"completions": fmt.Sprintf("%d/%d", j.Status.Succeeded, completions),
			"duration":    getJobDuration(j),
			"age":         formatAge(j.CreationTimestamp.Time),
			"labels":      getLabelsMap(j.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getJobDuration(j batchv1.Job) string {
	if j.Status.StartTime == nil {
		return "-"
	}
	if j.Status.CompletionTime != nil {
		return j.Status.CompletionTime.Sub(j.Status.StartTime.Time).Round(time.Second).String()
	}
	return time.Since(j.Status.StartTime.Time).Round(time.Second).String()
}

func getCronJobs(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedCronJobs
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, cj := range items {
		if !filterByNamespace(namespace, cj.Namespace) {
			continue
		}
		lastSchedule := "-"
		if cj.Status.LastScheduleTime != nil {
			lastSchedule = formatAge(cj.Status.LastScheduleTime.Time)
		}
		result = append(result, gin.H{
			"name":         cj.Name,
			"namespace":    cj.Namespace,
			"schedule":     cj.Spec.Schedule,
			"suspend":      *cj.Spec.Suspend,
			"active":       len(cj.Status.Active),
			"lastSchedule": lastSchedule,
			"age":          formatAge(cj.CreationTimestamp.Time),
			"labels":       getLabelsMap(cj.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getServices(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedServices
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, s := range items {
		if !filterByNamespace(namespace, s.Namespace) {
			continue
		}
		ports := make([]string, 0, len(s.Spec.Ports))
		for _, p := range s.Spec.Ports {
			if p.NodePort > 0 {
				ports = append(ports, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
			}
		}
		externalIP := "<none>"
		if len(s.Spec.ExternalIPs) > 0 {
			externalIP = s.Spec.ExternalIPs[0]
		} else if s.Spec.Type == corev1.ServiceTypeLoadBalancer && len(s.Status.LoadBalancer.Ingress) > 0 {
			externalIP = s.Status.LoadBalancer.Ingress[0].IP
		}
		result = append(result, gin.H{
			"name":       s.Name,
			"namespace":  s.Namespace,
			"type":       string(s.Spec.Type),
			"clusterIP":  s.Spec.ClusterIP,
			"externalIP": externalIP,
			"ports":      ports,
			"age":        formatAge(s.CreationTimestamp.Time),
			"labels":     getLabelsMap(s.Labels),
			"selector":   s.Spec.Selector,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getServiceDetail(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	cacheMu.RLock()
	services := cachedServices
	endpoints := cachedEndpoints
	cacheMu.RUnlock()

	var svc *corev1.Service
	for i := range services {
		if services[i].Namespace == namespace && services[i].Name == name {
			svc = &services[i]
			break
		}
	}

	if svc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	var ep *corev1.Endpoints
	for i := range endpoints {
		if endpoints[i].Namespace == namespace && endpoints[i].Name == name {
			ep = &endpoints[i]
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        svc.Name,
		"namespace":   svc.Namespace,
		"labels":      getLabelsMap(svc.Labels),
		"annotations": getLabelsMap(svc.Annotations),
		"type":        string(svc.Spec.Type),
		"clusterIP":   svc.Spec.ClusterIP,
		"ports":       svc.Spec.Ports,
		"selector":    svc.Spec.Selector,
		"endpoints":   ep,
	})
}

func getIngresses(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedIngresses
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, i := range items {
		if !filterByNamespace(namespace, i.Namespace) {
			continue
		}
		hosts := make([]string, 0)
		for _, rule := range i.Spec.Rules {
			hosts = append(hosts, rule.Host)
		}
		address := ""
		if len(i.Status.LoadBalancer.Ingress) > 0 {
			address = i.Status.LoadBalancer.Ingress[0].IP
			if address == "" {
				address = i.Status.LoadBalancer.Ingress[0].Hostname
			}
		}
		result = append(result, gin.H{
			"name":      i.Name,
			"namespace": i.Namespace,
			"class":     getIngressClassName(i),
			"hosts":     hosts,
			"address":   address,
			"age":       formatAge(i.CreationTimestamp.Time),
			"labels":    getLabelsMap(i.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getIngressClassName(i networkingv1.Ingress) string {
	if i.Spec.IngressClassName != nil {
		return *i.Spec.IngressClassName
	}
	return ""
}

func getIngressClasses(c *gin.Context) {
	cacheMu.RLock()
	items := cachedIngressClasses
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, ic := range items {
		result = append(result, gin.H{
			"name":       ic.Name,
			"controller": ic.Spec.Controller,
			"age":        formatAge(ic.CreationTimestamp.Time),
			"labels":     getLabelsMap(ic.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getEndpoints(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedEndpoints
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, e := range items {
		if !filterByNamespace(namespace, e.Namespace) {
			continue
		}
		endpoints := make([]string, 0)
		for _, subset := range e.Subsets {
			for _, addr := range subset.Addresses {
				for _, port := range subset.Ports {
					endpoints = append(endpoints, fmt.Sprintf("%s:%d", addr.IP, port.Port))
				}
			}
		}
		result = append(result, gin.H{
			"name":      e.Name,
			"namespace": e.Namespace,
			"endpoints": endpoints,
			"age":       formatAge(e.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getConfigMaps(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedConfigMaps
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, cm := range items {
		if !filterByNamespace(namespace, cm.Namespace) {
			continue
		}
		result = append(result, gin.H{
			"name":      cm.Name,
			"namespace": cm.Namespace,
			"data":      len(cm.Data),
			"age":       formatAge(cm.CreationTimestamp.Time),
			"labels":    getLabelsMap(cm.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getConfigMapDetail(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	cacheMu.RLock()
	items := cachedConfigMaps
	cacheMu.RUnlock()

	var cm *corev1.ConfigMap
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			cm = &items[i]
			break
		}
	}

	if cm == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ConfigMap not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        cm.Name,
		"namespace":   cm.Namespace,
		"labels":      getLabelsMap(cm.Labels),
		"annotations": getLabelsMap(cm.Annotations),
		"data":        cm.Data,
	})
}

func getSecrets(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedSecrets
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, s := range items {
		if !filterByNamespace(namespace, s.Namespace) {
			continue
		}
		result = append(result, gin.H{
			"name":      s.Name,
			"namespace": s.Namespace,
			"type":      string(s.Type),
			"data":      len(s.Data),
			"age":       formatAge(s.CreationTimestamp.Time),
			"labels":    getLabelsMap(s.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getSecretDetail(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	cacheMu.RLock()
	items := cachedSecrets
	cacheMu.RUnlock()

	var secret *corev1.Secret
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			secret = &items[i]
			break
		}
	}

	if secret == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
		return
	}

	// Return keys only, not values for security
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        secret.Name,
		"namespace":   secret.Namespace,
		"type":        string(secret.Type),
		"labels":      getLabelsMap(secret.Labels),
		"annotations": getLabelsMap(secret.Annotations),
		"keys":        keys,
	})
}

func getPersistentVolumeClaims(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedPVCs
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, pvc := range items {
		if !filterByNamespace(namespace, pvc.Namespace) {
			continue
		}
		storageClass := ""
		if pvc.Spec.StorageClassName != nil {
			storageClass = *pvc.Spec.StorageClassName
		}
		capacity := ""
		if pvc.Status.Capacity != nil {
			if storage, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
				capacity = storage.String()
			}
		}
		result = append(result, gin.H{
			"name":         pvc.Name,
			"namespace":    pvc.Namespace,
			"status":       string(pvc.Status.Phase),
			"volume":       pvc.Spec.VolumeName,
			"capacity":     capacity,
			"accessModes":  pvc.Spec.AccessModes,
			"storageClass": storageClass,
			"age":          formatAge(pvc.CreationTimestamp.Time),
			"labels":       getLabelsMap(pvc.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getPersistentVolumes(c *gin.Context) {
	cacheMu.RLock()
	items := cachedPVs
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, pv := range items {
		claim := ""
		if pv.Spec.ClaimRef != nil {
			claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		}
		result = append(result, gin.H{
			"name":            pv.Name,
			"capacity":        pv.Spec.Capacity.Storage().String(),
			"accessModes":     pv.Spec.AccessModes,
			"reclaimPolicy":   string(pv.Spec.PersistentVolumeReclaimPolicy),
			"status":          string(pv.Status.Phase),
			"claim":           claim,
			"storageClass":    pv.Spec.StorageClassName,
			"reason":          pv.Status.Reason,
			"age":             formatAge(pv.CreationTimestamp.Time),
			"labels":          getLabelsMap(pv.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getStorageClasses(c *gin.Context) {
	cacheMu.RLock()
	items := cachedStorageClasses
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, sc := range items {
		isDefault := false
		if v, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]; ok && v == "true" {
			isDefault = true
		}
		result = append(result, gin.H{
			"name":          sc.Name,
			"provisioner":   sc.Provisioner,
			"reclaimPolicy": string(*sc.ReclaimPolicy),
			"volumeBinding": string(*sc.VolumeBindingMode),
			"allowExpansion": sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion,
			"isDefault":     isDefault,
			"age":           formatAge(sc.CreationTimestamp.Time),
			"labels":        getLabelsMap(sc.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getClusterRoles(c *gin.Context) {
	cacheMu.RLock()
	items := cachedClusterRoles
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, cr := range items {
		result = append(result, gin.H{
			"name":   cr.Name,
			"rules":  len(cr.Rules),
			"age":    formatAge(cr.CreationTimestamp.Time),
			"labels": getLabelsMap(cr.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getClusterRoleBindings(c *gin.Context) {
	cacheMu.RLock()
	items := cachedClusterRoleBindings
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, crb := range items {
		result = append(result, gin.H{
			"name":     crb.Name,
			"role":     crb.RoleRef.Name,
			"subjects": len(crb.Subjects),
			"age":      formatAge(crb.CreationTimestamp.Time),
			"labels":   getLabelsMap(crb.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getServiceAccounts(c *gin.Context) {
	namespace := c.Query("namespace")
	cacheMu.RLock()
	items := cachedServiceAccounts
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, sa := range items {
		if !filterByNamespace(namespace, sa.Namespace) {
			continue
		}
		result = append(result, gin.H{
			"name":      sa.Name,
			"namespace": sa.Namespace,
			"secrets":   len(sa.Secrets),
			"age":       formatAge(sa.CreationTimestamp.Time),
			"labels":    getLabelsMap(sa.Labels),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func streamLogs(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")
	container := c.Query("container")

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	tailLines := int64(100)
	podLogOpts := &corev1.PodLogOptions{
		Follow:    true,
		TailLines: &tailLines,
	}
	if container != "" {
		podLogOpts.Container = container
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
	stream, err := req.Stream(context.Background())
	if err != nil {
		c.SSEvent("error", fmt.Sprintf("Failed to get logs: %v", err))
		return
	}
	defer stream.Close()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	go func() {
		<-ctx.Done()
		stream.Close()
	}()

	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return
			}
			c.SSEvent("log", line)
			c.Writer.Flush()
		}
	}
}

func getNodeMetrics(c *gin.Context) {
	if metricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Metrics server not available"})
		return
	}

	metrics, err := metricsClient.MetricsV1beta1().NodeMetricses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cacheMu.RLock()
	nodes := cachedNodes
	cacheMu.RUnlock()

	result := make([]gin.H, 0)
	for _, m := range metrics.Items {
		var node *corev1.Node
		for i := range nodes {
			if nodes[i].Name == m.Name {
				node = &nodes[i]
				break
			}
		}

		cpuUsage := m.Usage.Cpu().MilliValue()
		memUsage := m.Usage.Memory().Value()

		var cpuCapacity, memCapacity int64
		if node != nil {
			cpuCapacity = node.Status.Capacity.Cpu().MilliValue()
			memCapacity = node.Status.Capacity.Memory().Value()
		}

		result = append(result, gin.H{
			"name":        m.Name,
			"cpuUsage":    cpuUsage,
			"cpuCapacity": cpuCapacity,
			"cpuPercent":  float64(cpuUsage) / float64(cpuCapacity) * 100,
			"memUsage":    memUsage,
			"memCapacity": memCapacity,
			"memPercent":  float64(memUsage) / float64(memCapacity) * 100,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func getPodMetrics(c *gin.Context) {
	if metricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Metrics server not available"})
		return
	}

	namespace := c.Query("namespace")
	var metrics interface{}
	var err error

	if namespace != "" {
		metrics, err = metricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.Background(), metav1.ListOptions{})
	} else {
		metrics, err = metricsClient.MetricsV1beta1().PodMetricses("").List(context.Background(), metav1.ListOptions{})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// Ensure resource.Quantity is used to prevent import error
var _ = resource.Quantity{}
