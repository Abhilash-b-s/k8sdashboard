package k8s

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// ClusterClient holds all clients for a single cluster
type ClusterClient struct {
	Name            string
	Clientset       kubernetes.Interface
	MetricsClient   *metricsv.Clientset
	InformerFactory informers.SharedInformerFactory
	RestConfig      *rest.Config
	StopCh          chan struct{}
}

// MultiClusterManager manages connections to multiple Kubernetes clusters
type MultiClusterManager struct {
	clusters map[string]*ClusterClient
	mu       sync.RWMutex
}

// Manager is the global multi-cluster manager
var Manager *MultiClusterManager

// KubeconfigDir is the directory where uploaded kubeconfigs are stored
var KubeconfigDir = "./kubeconfigs"

// SetKubeconfigDir sets the directory for storing kubeconfigs
func SetKubeconfigDir(dir string) {
	KubeconfigDir = dir
}

// EnsureKubeconfigDir creates the kubeconfig directory if it doesn't exist
func EnsureKubeconfigDir() error {
	return os.MkdirAll(KubeconfigDir, 0755)
}

// InitEmptyManager initializes an empty multi-cluster manager
// This allows kubeconfig uploads even when no initial clusters are available
func InitEmptyManager() {
	Manager = &MultiClusterManager{
		clusters: make(map[string]*ClusterClient),
	}
	log.Println("Initialized empty multi-cluster manager")
}

// InitInClusterClient tries to connect to the cluster using in-cluster config (service account)
// This is used when running inside a Kubernetes cluster
func InitInClusterClient() error {
	if Manager == nil {
		Manager = &MultiClusterManager{
			clusters: make(map[string]*ClusterClient),
		}
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("not running in cluster or service account not available: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		log.Printf("Warning: metrics not available for in-cluster: %v", err)
	}

	informerFactory := informers.NewSharedInformerFactory(clientset, time.Minute*10)
	stopCh := make(chan struct{})

	// Start all informers needed by the dashboard
	// Core
	informerFactory.Core().V1().Pods().Informer()
	informerFactory.Core().V1().Nodes().Informer()
	informerFactory.Core().V1().Services().Informer()
	informerFactory.Core().V1().Namespaces().Informer()
	informerFactory.Core().V1().ConfigMaps().Informer()
	informerFactory.Core().V1().Secrets().Informer()
	informerFactory.Core().V1().ServiceAccounts().Informer()
	informerFactory.Core().V1().PersistentVolumes().Informer()
	informerFactory.Core().V1().PersistentVolumeClaims().Informer()
	informerFactory.Core().V1().Events().Informer()

	// Apps
	informerFactory.Apps().V1().Deployments().Informer()
	informerFactory.Apps().V1().DaemonSets().Informer()
	informerFactory.Apps().V1().StatefulSets().Informer()
	informerFactory.Apps().V1().ReplicaSets().Informer()

	// Batch
	informerFactory.Batch().V1().Jobs().Informer()
	informerFactory.Batch().V1().CronJobs().Informer()

	// Networking
	informerFactory.Networking().V1().Ingresses().Informer()
	informerFactory.Networking().V1().IngressClasses().Informer()
	informerFactory.Networking().V1().NetworkPolicies().Informer()

	// Storage
	informerFactory.Storage().V1().StorageClasses().Informer()

	// RBAC
	informerFactory.Rbac().V1().Roles().Informer()
	informerFactory.Rbac().V1().RoleBindings().Informer()
	informerFactory.Rbac().V1().ClusterRoles().Informer()
	informerFactory.Rbac().V1().ClusterRoleBindings().Informer()

	// Autoscaling
	informerFactory.Autoscaling().V2().HorizontalPodAutoscalers().Informer()

	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	client := &ClusterClient{
		Name:            "default",
		Clientset:       clientset,
		MetricsClient:   metricsClient,
		InformerFactory: informerFactory,
		RestConfig:      config,
		StopCh:          stopCh,
	}

	Manager.mu.Lock()
	Manager.clusters["default"] = client
	Manager.mu.Unlock()

	log.Println("✓ Connected to in-cluster (default)")
	return nil
}

// InitMultiCluster initializes clients for all contexts in the kubeconfig
func InitMultiCluster(kubeconfigPath string) error {
	// Always ensure Manager is initialized
	if Manager == nil {
		Manager = &MultiClusterManager{
			clusters: make(map[string]*ClusterClient),
		}
	}

	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		log.Printf("Warning: failed to load kubeconfig from %s: %v", kubeconfigPath, err)
		return nil // Don't return error, just log warning - Manager is still usable
	}

	// Create client for each context
	connectedCount := 0
	for contextName := range config.Contexts {
		client, err := createClientForContext(kubeconfigPath, contextName)
		if err != nil {
			log.Printf("Warning: failed to connect to cluster '%s': %v", contextName, err)
			continue
		}
		Manager.clusters[contextName] = client
		log.Printf("✓ Connected to cluster: %s", contextName)
		connectedCount++
	}

	if connectedCount == 0 {
		log.Println("Warning: no clusters could be connected from initial kubeconfig")
	} else {
		log.Printf("Successfully connected to %d cluster(s)", connectedCount)
	}

	return nil
}

func createClientForContext(kubeconfigPath, contextName string) (*ClusterClient, error) {
	// Build config for this specific context
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		configOverrides,
	)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	// Create Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Test connection
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Create metrics client (optional)
	metricsClient, _ := metricsv.NewForConfig(restConfig)

	// Create informer factory
	informerFactory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

	// Start informers for this cluster
	stopCh := make(chan struct{})
	startClusterInformers(informerFactory, stopCh)

	return &ClusterClient{
		Name:            contextName,
		Clientset:       clientset,
		MetricsClient:   metricsClient,
		InformerFactory: informerFactory,
		RestConfig:      restConfig,
		StopCh:          stopCh,
	}, nil
}

func startClusterInformers(factory informers.SharedInformerFactory, stopCh chan struct{}) {
	// Core resources
	_ = factory.Core().V1().Nodes().Informer()
	_ = factory.Core().V1().Namespaces().Informer()
	_ = factory.Core().V1().Pods().Informer()
	_ = factory.Core().V1().Services().Informer()
	_ = factory.Core().V1().Endpoints().Informer()
	_ = factory.Core().V1().ConfigMaps().Informer()
	_ = factory.Core().V1().Secrets().Informer()
	_ = factory.Core().V1().PersistentVolumes().Informer()
	_ = factory.Core().V1().PersistentVolumeClaims().Informer()
	_ = factory.Core().V1().ServiceAccounts().Informer()
	_ = factory.Core().V1().ReplicationControllers().Informer()
	_ = factory.Core().V1().Events().Informer()

	// Apps resources
	_ = factory.Apps().V1().Deployments().Informer()
	_ = factory.Apps().V1().DaemonSets().Informer()
	_ = factory.Apps().V1().StatefulSets().Informer()
	_ = factory.Apps().V1().ReplicaSets().Informer()

	// Batch resources
	_ = factory.Batch().V1().Jobs().Informer()
	_ = factory.Batch().V1().CronJobs().Informer()

	// Networking resources
	_ = factory.Networking().V1().Ingresses().Informer()
	_ = factory.Networking().V1().IngressClasses().Informer()
	_ = factory.Networking().V1().NetworkPolicies().Informer()

	// Storage resources
	_ = factory.Storage().V1().StorageClasses().Informer()

	// RBAC resources
	_ = factory.Rbac().V1().ClusterRoles().Informer()
	_ = factory.Rbac().V1().ClusterRoleBindings().Informer()
	_ = factory.Rbac().V1().Roles().Informer()
	_ = factory.Rbac().V1().RoleBindings().Informer()

	// Autoscaling resources
	_ = factory.Autoscaling().V2().HorizontalPodAutoscalers().Informer()

	// Start and sync
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)
}

// GetClient returns the client for a specific cluster
func (m *MultiClusterManager) GetClient(clusterName string) (*ClusterClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clusters[clusterName]
	if !ok {
		return nil, fmt.Errorf("cluster '%s' not found", clusterName)
	}
	return client, nil
}

// ListClusters returns all available cluster names
func (m *MultiClusterManager) ListClusters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clusters))
	for name := range m.clusters {
		names = append(names, name)
	}
	return names
}

// GetAllClients returns all cluster clients
func (m *MultiClusterManager) GetAllClients() map[string]*ClusterClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	clients := make(map[string]*ClusterClient, len(m.clusters))
	for k, v := range m.clusters {
		clients[k] = v
	}
	return clients
}

// ClusterCount returns the number of connected clusters
func (m *MultiClusterManager) ClusterCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clusters)
}

// Shutdown gracefully stops all cluster connections
func (m *MultiClusterManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clusters {
		close(client.StopCh)
		log.Printf("Disconnected from cluster: %s", name)
	}
}

// AddClustersFromKubeconfig loads clusters from a new kubeconfig file
// Returns the list of newly added clusters and any errors
func (m *MultiClusterManager) AddClustersFromKubeconfig(kubeconfigPath string) ([]string, []string, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var added []string
	var failed []string

	for contextName := range config.Contexts {
		// Skip if cluster already exists
		if _, exists := m.clusters[contextName]; exists {
			log.Printf("Cluster '%s' already exists, skipping", contextName)
			continue
		}

		client, err := createClientForContext(kubeconfigPath, contextName)
		if err != nil {
			log.Printf("Warning: failed to connect to cluster '%s': %v", contextName, err)
			failed = append(failed, contextName)
			continue
		}

		m.clusters[contextName] = client
		added = append(added, contextName)
		log.Printf("✓ Added cluster: %s", contextName)
	}

	return added, failed, nil
}

// AddCluster adds a single cluster from a kubeconfig file with a specific context
func (m *MultiClusterManager) AddCluster(kubeconfigPath, contextName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if cluster already exists
	if _, exists := m.clusters[contextName]; exists {
		return fmt.Errorf("cluster '%s' already exists", contextName)
	}

	client, err := createClientForContext(kubeconfigPath, contextName)
	if err != nil {
		return fmt.Errorf("failed to add cluster '%s': %w", contextName, err)
	}

	m.clusters[contextName] = client
	log.Printf("✓ Added cluster: %s", contextName)
	return nil
}

// RemoveCluster removes a cluster connection
func (m *MultiClusterManager) RemoveCluster(clusterName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clusters[clusterName]
	if !exists {
		return fmt.Errorf("cluster '%s' not found", clusterName)
	}

	close(client.StopCh)
	delete(m.clusters, clusterName)
	log.Printf("Removed cluster: %s", clusterName)
	return nil
}

// GetContextsFromKubeconfig returns available contexts from a kubeconfig file without connecting
func GetContextsFromKubeconfig(kubeconfigPath string) ([]string, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	contexts := make([]string, 0, len(config.Contexts))
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}
	return contexts, nil
}

// GetContextsFromKubeconfigContent returns available contexts from kubeconfig content without connecting
func GetContextsFromKubeconfigContent(content []byte) ([]string, error) {
	config, err := clientcmd.Load(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	contexts := make([]string, 0, len(config.Contexts))
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}
	return contexts, nil
}

// AddClustersFromKubeconfigContent loads clusters from kubeconfig content
// nameMapping optionally maps original context names to custom display names
func (m *MultiClusterManager) AddClustersFromKubeconfigContent(content []byte, nameMapping map[string]string) ([]string, []string, error) {
	config, err := clientcmd.Load(content)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var added []string
	var failed []string

	for contextName := range config.Contexts {
		// Determine the name to use (custom name if provided, otherwise context name)
		clusterName := contextName
		if nameMapping != nil {
			if customName, ok := nameMapping[contextName]; ok && customName != "" {
				clusterName = customName
			}
		}

		// Skip if cluster already exists
		if _, exists := m.clusters[clusterName]; exists {
			log.Printf("Cluster '%s' already exists, skipping", clusterName)
			continue
		}

		client, err := createClientFromConfig(config, contextName)
		if err != nil {
			log.Printf("Warning: failed to connect to cluster '%s': %v", contextName, err)
			failed = append(failed, clusterName)
			continue
		}

		client.Name = clusterName // Set the custom name
		m.clusters[clusterName] = client
		added = append(added, clusterName)
		log.Printf("✓ Added cluster: %s (context: %s)", clusterName, contextName)
	}

	return added, failed, nil
}

func createClientFromConfig(config *clientcmdapi.Config, contextName string) (*ClusterClient, error) {
	// Build client config for this specific context
	clientConfig := clientcmd.NewNonInteractiveClientConfig(*config, contextName, &clientcmd.ConfigOverrides{}, nil)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	// Create Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Test connection
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Create metrics client (optional)
	metricsClient, _ := metricsv.NewForConfig(restConfig)

	// Create informer factory
	informerFactory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

	// Start informers for this cluster
	stopCh := make(chan struct{})
	startClusterInformers(informerFactory, stopCh)

	return &ClusterClient{
		Name:            contextName,
		Clientset:       clientset,
		MetricsClient:   metricsClient,
		InformerFactory: informerFactory,
		RestConfig:      restConfig,
		StopCh:          stopCh,
	}, nil
}

// SaveKubeconfig saves kubeconfig content to the storage directory
// Returns the filename it was saved as
func SaveKubeconfig(content []byte, name string) (string, error) {
	if err := EnsureKubeconfigDir(); err != nil {
		return "", fmt.Errorf("failed to create kubeconfig directory: %w", err)
	}

	// Sanitize the name for use as a filename
	safeName := strings.ReplaceAll(name, "/", "-")
	safeName = strings.ReplaceAll(safeName, "\\", "-")
	safeName = strings.ReplaceAll(safeName, " ", "_")
	if !strings.HasSuffix(safeName, ".yaml") && !strings.HasSuffix(safeName, ".yml") {
		safeName = safeName + ".yaml"
	}

	filePath := filepath.Join(KubeconfigDir, safeName)

	if err := os.WriteFile(filePath, content, 0600); err != nil {
		return "", fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	log.Printf("✓ Saved kubeconfig to: %s", filePath)
	return safeName, nil
}

// LoadStoredKubeconfigs loads all kubeconfigs from the storage directory
func LoadStoredKubeconfigs() error {
	if err := EnsureKubeconfigDir(); err != nil {
		return err
	}

	files, err := os.ReadDir(KubeconfigDir)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".conf") {
			continue
		}

		filePath := filepath.Join(KubeconfigDir, name)
		log.Printf("Loading kubeconfig from: %s", filePath)

		_, _, err := Manager.AddClustersFromKubeconfig(filePath)
		if err != nil {
			log.Printf("Warning: failed to load kubeconfig %s: %v", name, err)
		}
	}

	return nil
}

// ListStoredKubeconfigs returns the list of stored kubeconfig files
func ListStoredKubeconfigs() ([]string, error) {
	if err := EnsureKubeconfigDir(); err != nil {
		return nil, err
	}

	files, err := os.ReadDir(KubeconfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig directory: %w", err)
	}

	var configs []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".conf") {
			configs = append(configs, name)
		}
	}

	return configs, nil
}

// DeleteStoredKubeconfig removes a stored kubeconfig file
func DeleteStoredKubeconfig(name string) error {
	filePath := filepath.Join(KubeconfigDir, name)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("kubeconfig file '%s' not found", name)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete kubeconfig: %w", err)
	}

	log.Printf("✓ Deleted kubeconfig: %s", filePath)
	return nil
}
