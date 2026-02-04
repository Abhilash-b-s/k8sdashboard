package k8s

import (
	"log"
	"time"

	"k8s.io/client-go/informers"
)

var InformerFactory informers.SharedInformerFactory
var StopCh = make(chan struct{})

// StartInformers initializes and starts the SharedInformerFactory
func StartInformers() {
	// Create informer factory with 30-second resync period
	InformerFactory = informers.NewSharedInformerFactory(Clientset, 30*time.Second)

	// Core resources
	_ = InformerFactory.Core().V1().Nodes().Informer()
	_ = InformerFactory.Core().V1().Namespaces().Informer()
	_ = InformerFactory.Core().V1().Pods().Informer()
	_ = InformerFactory.Core().V1().Services().Informer()
	_ = InformerFactory.Core().V1().Endpoints().Informer()
	_ = InformerFactory.Core().V1().ConfigMaps().Informer()
	_ = InformerFactory.Core().V1().Secrets().Informer()
	_ = InformerFactory.Core().V1().PersistentVolumes().Informer()
	_ = InformerFactory.Core().V1().PersistentVolumeClaims().Informer()
	_ = InformerFactory.Core().V1().ServiceAccounts().Informer()
	_ = InformerFactory.Core().V1().ReplicationControllers().Informer()

	// Apps resources
	_ = InformerFactory.Apps().V1().Deployments().Informer()
	_ = InformerFactory.Apps().V1().DaemonSets().Informer()
	_ = InformerFactory.Apps().V1().StatefulSets().Informer()
	_ = InformerFactory.Apps().V1().ReplicaSets().Informer()

	// Batch resources
	_ = InformerFactory.Batch().V1().Jobs().Informer()
	_ = InformerFactory.Batch().V1().CronJobs().Informer()

	// Networking resources
	_ = InformerFactory.Networking().V1().Ingresses().Informer()
	_ = InformerFactory.Networking().V1().IngressClasses().Informer()

	// Storage resources
	_ = InformerFactory.Storage().V1().StorageClasses().Informer()

	// RBAC resources
	_ = InformerFactory.Rbac().V1().ClusterRoles().Informer()
	_ = InformerFactory.Rbac().V1().ClusterRoleBindings().Informer()
	_ = InformerFactory.Rbac().V1().Roles().Informer()
	_ = InformerFactory.Rbac().V1().RoleBindings().Informer()

	// Networking resources - extended
	_ = InformerFactory.Networking().V1().NetworkPolicies().Informer()

	// Autoscaling resources
	_ = InformerFactory.Autoscaling().V2().HorizontalPodAutoscalers().Informer()

	// Events
	_ = InformerFactory.Core().V1().Events().Informer()

	// Start the informer factory
	InformerFactory.Start(StopCh)

	// Wait for caches to sync
	InformerFactory.WaitForCacheSync(StopCh)
	log.Println("Informer caches synced successfully")
}
