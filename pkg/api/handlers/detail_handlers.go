package handlers

import (
	"context"
	"fmt"
	"net/http"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceDetail represents detailed information about any resource
type ResourceDetail struct {
	Kind              string                 `json:"kind"`
	Name              string                 `json:"name"`
	Namespace         string                 `json:"namespace,omitempty"`
	UID               string                 `json:"uid"`
	CreationTimestamp string                 `json:"creationTimestamp"`
	Age               string                 `json:"age"`
	Labels            map[string]string      `json:"labels,omitempty"`
	Annotations       map[string]string      `json:"annotations,omitempty"`
	OwnerReferences   []OwnerReferenceInfo   `json:"ownerReferences,omitempty"`
	Spec              interface{}            `json:"spec,omitempty"`
	Status            interface{}            `json:"status,omitempty"`
	Raw               interface{}            `json:"raw,omitempty"`
}

// OwnerReferenceInfo represents owner reference information
type OwnerReferenceInfo struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	UID  string `json:"uid"`
}

// ContainerDetail represents detailed container information
type ContainerDetail struct {
	Name            string            `json:"name"`
	Image           string            `json:"image"`
	ImagePullPolicy string            `json:"imagePullPolicy"`
	Ports           []ContainerPort   `json:"ports,omitempty"`
	Env             []EnvVar          `json:"env,omitempty"`
	VolumeMounts    []VolumeMount     `json:"volumeMounts,omitempty"`
	Resources       ResourceRequirements `json:"resources,omitempty"`
	Command         []string          `json:"command,omitempty"`
	Args            []string          `json:"args,omitempty"`
	WorkingDir      string            `json:"workingDir,omitempty"`
	State           string            `json:"state,omitempty"`
	Ready           bool              `json:"ready"`
	RestartCount    int32             `json:"restartCount"`
}

// ContainerPort represents a port exposed by a container
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// VolumeMount represents a volume mount
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly"`
}

// ResourceRequirements represents resource requests and limits
type ResourceRequirements struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

// PodDetailResponse represents detailed pod information
type PodDetailResponse struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	UID             string            `json:"uid"`
	Node            string            `json:"node"`
	Status          string            `json:"status"`
	PodIP           string            `json:"podIP"`
	HostIP          string            `json:"hostIP"`
	QOSClass        string            `json:"qosClass"`
	ServiceAccount  string            `json:"serviceAccount"`
	RestartPolicy   string            `json:"restartPolicy"`
	Labels          map[string]string `json:"labels"`
	Annotations     map[string]string `json:"annotations"`
	OwnerReferences []OwnerReferenceInfo `json:"ownerReferences"`
	Conditions      []ConditionInfo   `json:"conditions"`
	Containers      []ContainerDetail `json:"containers"`
	InitContainers  []ContainerDetail `json:"initContainers"`
	Volumes         []VolumeInfo      `json:"volumes"`
	CreatedAt       string            `json:"createdAt"`
	Age             string            `json:"age"`
}

// ConditionInfo represents a condition
type ConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// VolumeInfo represents volume information
type VolumeInfo struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Source string `json:"source"`
}

// GetPodDetail returns detailed pod information
func GetPodDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	pod, err := k8s.InformerFactory.Core().V1().Pods().Lister().Pods(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	// Build container details
	containers := make([]ContainerDetail, 0)
	for i, container := range pod.Spec.Containers {
		cd := ContainerDetail{
			Name:            container.Name,
			Image:           container.Image,
			ImagePullPolicy: string(container.ImagePullPolicy),
			Command:         container.Command,
			Args:            container.Args,
			WorkingDir:      container.WorkingDir,
		}

		// Ports
		for _, port := range container.Ports {
			cd.Ports = append(cd.Ports, ContainerPort{
				Name:          port.Name,
				ContainerPort: port.ContainerPort,
				Protocol:      string(port.Protocol),
			})
		}

		// Env vars (only names, not values for security)
		for _, env := range container.Env {
			cd.Env = append(cd.Env, EnvVar{Name: env.Name, Value: "[hidden]"})
		}

		// Volume mounts
		for _, vm := range container.VolumeMounts {
			cd.VolumeMounts = append(cd.VolumeMounts, VolumeMount{
				Name:      vm.Name,
				MountPath: vm.MountPath,
				ReadOnly:  vm.ReadOnly,
			})
		}

		// Resources
		cd.Resources = ResourceRequirements{
			Requests: make(map[string]string),
			Limits:   make(map[string]string),
		}
		for k, v := range container.Resources.Requests {
			cd.Resources.Requests[string(k)] = v.String()
		}
		for k, v := range container.Resources.Limits {
			cd.Resources.Limits[string(k)] = v.String()
		}

		// Container status
		if i < len(pod.Status.ContainerStatuses) {
			status := pod.Status.ContainerStatuses[i]
			cd.Ready = status.Ready
			cd.RestartCount = status.RestartCount
			if status.State.Running != nil {
				cd.State = "Running"
			} else if status.State.Waiting != nil {
				cd.State = fmt.Sprintf("Waiting: %s", status.State.Waiting.Reason)
			} else if status.State.Terminated != nil {
				cd.State = fmt.Sprintf("Terminated: %s", status.State.Terminated.Reason)
			}
		}

		containers = append(containers, cd)
	}

	// Build init container details
	initContainers := make([]ContainerDetail, 0)
	for _, container := range pod.Spec.InitContainers {
		cd := ContainerDetail{
			Name:            container.Name,
			Image:           container.Image,
			ImagePullPolicy: string(container.ImagePullPolicy),
			Command:         container.Command,
			Args:            container.Args,
		}
		initContainers = append(initContainers, cd)
	}

	// Build volume info
	volumes := make([]VolumeInfo, 0)
	for _, vol := range pod.Spec.Volumes {
		vi := VolumeInfo{Name: vol.Name}
		if vol.ConfigMap != nil {
			vi.Type = "ConfigMap"
			vi.Source = vol.ConfigMap.Name
		} else if vol.Secret != nil {
			vi.Type = "Secret"
			vi.Source = vol.Secret.SecretName
		} else if vol.PersistentVolumeClaim != nil {
			vi.Type = "PVC"
			vi.Source = vol.PersistentVolumeClaim.ClaimName
		} else if vol.EmptyDir != nil {
			vi.Type = "EmptyDir"
			vi.Source = "-"
		} else if vol.HostPath != nil {
			vi.Type = "HostPath"
			vi.Source = vol.HostPath.Path
		} else {
			vi.Type = "Other"
			vi.Source = "-"
		}
		volumes = append(volumes, vi)
	}

	// Build owner references
	ownerRefs := make([]OwnerReferenceInfo, 0)
	for _, ref := range pod.OwnerReferences {
		ownerRefs = append(ownerRefs, OwnerReferenceInfo{
			Kind: ref.Kind,
			Name: ref.Name,
			UID:  string(ref.UID),
		})
	}

	// Build conditions
	conditions := make([]ConditionInfo, 0)
	for _, cond := range pod.Status.Conditions {
		conditions = append(conditions, ConditionInfo{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	response := PodDetailResponse{
		Name:            pod.Name,
		Namespace:       pod.Namespace,
		UID:             string(pod.UID),
		Node:            pod.Spec.NodeName,
		Status:          string(pod.Status.Phase),
		PodIP:           pod.Status.PodIP,
		HostIP:          pod.Status.HostIP,
		QOSClass:        string(pod.Status.QOSClass),
		ServiceAccount:  pod.Spec.ServiceAccountName,
		RestartPolicy:   string(pod.Spec.RestartPolicy),
		Labels:          pod.Labels,
		Annotations:     pod.Annotations,
		OwnerReferences: ownerRefs,
		Conditions:      conditions,
		Containers:      containers,
		InitContainers:  initContainers,
		Volumes:         volumes,
		CreatedAt:       pod.CreationTimestamp.Format("2006-01-02 15:04:05"),
		Age:             FormatAge(pod.CreationTimestamp.Time),
	}

	c.JSON(http.StatusOK, response)
}

// GetDaemonSetDetail returns detailed daemonset information
func GetDaemonSetDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	ds, err := k8s.InformerFactory.Apps().V1().DaemonSets().Lister().DaemonSets(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DaemonSet not found"})
		return
	}
	c.JSON(http.StatusOK, ds)
}

// GetStatefulSetDetail returns detailed statefulset information
func GetStatefulSetDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	ss, err := k8s.InformerFactory.Apps().V1().StatefulSets().Lister().StatefulSets(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "StatefulSet not found"})
		return
	}
	c.JSON(http.StatusOK, ss)
}

// GetReplicaSetDetail returns detailed replicaset information
func GetReplicaSetDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	rs, err := k8s.InformerFactory.Apps().V1().ReplicaSets().Lister().ReplicaSets(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ReplicaSet not found"})
		return
	}
	c.JSON(http.StatusOK, rs)
}

// GetJobDetail returns detailed job information
func GetJobDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	job, err := k8s.InformerFactory.Batch().V1().Jobs().Lister().Jobs(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// GetCronJobDetail returns detailed cronjob information
func GetCronJobDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	cj, err := k8s.InformerFactory.Batch().V1().CronJobs().Lister().CronJobs(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CronJob not found"})
		return
	}
	c.JSON(http.StatusOK, cj)
}

// GetIngressDetail returns detailed ingress information
func GetIngressDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	ing, err := k8s.InformerFactory.Networking().V1().Ingresses().Lister().Ingresses(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ingress not found"})
		return
	}
	c.JSON(http.StatusOK, ing)
}

// GetPVDetail returns detailed persistent volume information
func GetPVDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	name := c.Param("name")

	pv, err := k8s.InformerFactory.Core().V1().PersistentVolumes().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PersistentVolume not found"})
		return
	}
	c.JSON(http.StatusOK, pv)
}

// GetPVCDetail returns detailed persistent volume claim information
func GetPVCDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	pvc, err := k8s.InformerFactory.Core().V1().PersistentVolumeClaims().Lister().PersistentVolumeClaims(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PersistentVolumeClaim not found"})
		return
	}
	c.JSON(http.StatusOK, pvc)
}

// GetStorageClassDetail returns detailed storage class information
func GetStorageClassDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	name := c.Param("name")

	sc, err := k8s.InformerFactory.Storage().V1().StorageClasses().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "StorageClass not found"})
		return
	}
	c.JSON(http.StatusOK, sc)
}

// GetClusterRoleDetail returns detailed cluster role information
func GetClusterRoleDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	name := c.Param("name")

	cr, err := k8s.InformerFactory.Rbac().V1().ClusterRoles().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ClusterRole not found"})
		return
	}
	c.JSON(http.StatusOK, cr)
}

// GetClusterRoleBindingDetail returns detailed cluster role binding information
func GetClusterRoleBindingDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	name := c.Param("name")

	crb, err := k8s.InformerFactory.Rbac().V1().ClusterRoleBindings().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ClusterRoleBinding not found"})
		return
	}
	c.JSON(http.StatusOK, crb)
}

// GetRoleDetail returns detailed role information
func GetRoleDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	role, err := k8s.InformerFactory.Rbac().V1().Roles().Lister().Roles(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}
	c.JSON(http.StatusOK, role)
}

// GetRoleBindingDetail returns detailed role binding information
func GetRoleBindingDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	rb, err := k8s.InformerFactory.Rbac().V1().RoleBindings().Lister().RoleBindings(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "RoleBinding not found"})
		return
	}
	c.JSON(http.StatusOK, rb)
}

// GetServiceAccountDetail returns detailed service account information
func GetServiceAccountDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	sa, err := k8s.InformerFactory.Core().V1().ServiceAccounts().Lister().ServiceAccounts(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
		return
	}
	c.JSON(http.StatusOK, sa)
}

// GetHPADetail returns detailed HPA information
func GetHPADetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	hpa, err := k8s.InformerFactory.Autoscaling().V2().HorizontalPodAutoscalers().Lister().HorizontalPodAutoscalers(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "HPA not found"})
		return
	}
	c.JSON(http.StatusOK, hpa)
}

// GetNetworkPolicyDetail returns detailed network policy information
func GetNetworkPolicyDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	np, err := k8s.InformerFactory.Networking().V1().NetworkPolicies().Lister().NetworkPolicies(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "NetworkPolicy not found"})
		return
	}
	c.JSON(http.StatusOK, np)
}

// GetNamespaceDetail returns detailed namespace information
func GetNamespaceDetail(c *gin.Context) {
	name := c.Param("name")

	ns, err := k8s.Clientset.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Namespace not found"})
		return
	}
	c.JSON(http.StatusOK, ns)
}
