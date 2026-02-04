package handlers

import (
	"time"
	"fmt"
)

// OverviewResponse represents cluster overview stats
type OverviewResponse struct {
	TotalNodes       int      `json:"totalNodes"`
	ReadyNodes       int      `json:"readyNodes"`
	TotalPods        int      `json:"totalPods"`
	RunningPods      int      `json:"runningPods"`
	PendingPods      int      `json:"pendingPods"`
	FailedPods       int      `json:"failedPods"`
	TotalDeployments int      `json:"totalDeployments"`
	TotalServices    int      `json:"totalServices"`
	Namespaces       []string `json:"namespaces"`
}

// ResourcesResponse represents resources grouped by namespace
type ResourcesResponse struct {
	Namespaces map[string]NamespaceResources `json:"namespaces"`
}

// NamespaceResources holds pods and deployments for a namespace
type NamespaceResources struct {
	Pods        []PodInfo        `json:"pods"`
	Deployments []DeploymentInfo `json:"deployments"`
}

// PodInfo represents simplified pod information
type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Age       string `json:"age"`
	Node      string `json:"node"`
}

// DeploymentInfo represents simplified deployment information
type DeploymentInfo struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Ready      string `json:"ready"`
	UpToDate   int32  `json:"upToDate"`
	Available  int32  `json:"available"`
	Age        string `json:"age"`
}

// FormatAge returns a human-readable age string
func FormatAge(t time.Time) string {
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

// ServiceInfo represents simplified service information
type ServiceInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	ClusterIP string `json:"clusterIP"`
	Ports     string `json:"ports"`
	Age       string `json:"age"`
}

// IngressInfo represents simplified ingress information
type IngressInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Hosts     string `json:"hosts"`
	Address   string `json:"address"`
	Age       string `json:"age"`
}

// ConfigMapInfo represents simplified configmap information
type ConfigMapInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Keys      int    `json:"keys"`
	Age       string `json:"age"`
}

// SecretInfo represents simplified secret information
type SecretInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	Keys      int    `json:"keys"`
	Age       string `json:"age"`
}

// NodeInfo represents simplified node information
type NodeInfo struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Roles      []string `json:"roles"`
	Version    string   `json:"version"`
	InternalIP string   `json:"internalIP"`
	CPU        string   `json:"cpu"`
	Memory     string   `json:"memory"`
	Age        string   `json:"age"`
}

// NodeDetail represents detailed node information
type NodeDetail struct {
	NodeInfo
	Conditions []NodeCondition `json:"conditions"`
	Pods       []PodInfo       `json:"pods"`
	Labels     map[string]string `json:"labels"`
}

// NodeCondition represents a node condition
type NodeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type DaemonSetInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Desired   int32  `json:"desired"`
	Current   int32  `json:"current"`
	Ready     int32  `json:"ready"`
	UpToDate  int32  `json:"upToDate"`
	Available int32  `json:"available"`
	Age       string `json:"age"`
}

type StatefulSetInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Ready     string `json:"ready"` // e.g., "1/3"
	Age       string `json:"age"`
}

type ReplicaSetInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Desired   int32  `json:"desired"`
	Current   int32  `json:"current"`
	Ready     int32  `json:"ready"`
	Age       string `json:"age"`
}

type ReplicationControllerInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Desired   int32  `json:"desired"`
	Current   int32  `json:"current"`
	Ready     int32  `json:"ready"`
	Age       string `json:"age"`
}

type JobInfo struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Completions string `json:"completions"` // e.g., "1/1"
	Duration    string `json:"duration"`
	Age         string `json:"age"`
}

type CronJobInfo struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Schedule     string `json:"schedule"`
	Suspend      bool   `json:"suspend"`
	Active       int    `json:"active"`
	LastSchedule string `json:"lastSchedule"`
	Age          string `json:"age"`
}

type PVCInfo struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Status       string `json:"status"`
	Volume       string `json:"volume"`
	Capacity     string `json:"capacity"`
	StorageClass string `json:"storageClass"`
	Age          string `json:"age"`
}

type PVInfo struct {
	Name          string `json:"name"`
	Capacity      string `json:"capacity"`
	ReclaimPolicy string `json:"reclaimPolicy"`
	Status        string `json:"status"`
	Claim         string `json:"claim"`
	StorageClass  string `json:"storageClass"`
	Age           string `json:"age"`
}

type StorageClassInfo struct {
	Name           string `json:"name"`
	Provisioner    string `json:"provisioner"`
	ReclaimPolicy  string `json:"reclaimPolicy"`
	VolumeBinding  string `json:"volumeBinding"`
	AllowExpansion bool   `json:"allowExpansion"`
	IsDefault      bool   `json:"isDefault"`
	Age            string `json:"age"`
}

type ClusterRoleInfo struct {
	Name  string `json:"name"`
	Rules int    `json:"rules"` // Count of rules
	Age   string `json:"age"`
}

type ClusterRoleBindingInfo struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Subjects int    `json:"subjects"` // Count of subjects
	Age      string `json:"age"`
}

type IngressClassInfo struct {
	Name       string `json:"name"`
	Controller string `json:"controller"`
	Age        string `json:"age"`
}
