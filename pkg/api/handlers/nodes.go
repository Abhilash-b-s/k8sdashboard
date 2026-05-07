package handlers

import (
	"fmt"
	"net/http"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetNodes returns list of nodes
func GetNodes(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	nodes, err := k8s.InformerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list nodes"})
		return
	}

	var result []NodeInfo
	for _, node := range nodes {
		result = append(result, toNodeInfo(node))
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetNode returns node details
func GetNode(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	name := c.Param("name")
	node, err := k8s.InformerFactory.Core().V1().Nodes().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	// Get pods on this node and calculate resource usage
	pods, err := k8s.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	podCount := 0
	var cpuRequests, cpuLimits, memRequests, memLimits int64
	if err == nil {
		for _, pod := range pods {
			if pod.Spec.NodeName == name && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
				podCount++
				for _, container := range pod.Spec.Containers {
					if container.Resources.Requests != nil {
						cpuRequests += container.Resources.Requests.Cpu().MilliValue()
						memRequests += container.Resources.Requests.Memory().Value()
					}
					if container.Resources.Limits != nil {
						cpuLimits += container.Resources.Limits.Cpu().MilliValue()
						memLimits += container.Resources.Limits.Memory().Value()
					}
				}
			}
		}
	}

	// Conditions
	var conditions []NodeCondition
	for _, cond := range node.Status.Conditions {
		conditions = append(conditions, NodeCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	// Taints
	var taints []NodeTaint
	for _, t := range node.Spec.Taints {
		taints = append(taints, NodeTaint{
			Key:    t.Key,
			Value:  t.Value,
			Effect: string(t.Effect),
		})
	}

	// Addresses
	var addresses []NodeAddress
	for _, addr := range node.Status.Addresses {
		addresses = append(addresses, NodeAddress{
			Type:    string(addr.Type),
			Address: addr.Address,
		})
	}

	// Capacity
	capacity := make(map[string]string)
	for k, v := range node.Status.Capacity {
		capacity[string(k)] = v.String()
	}

	// Allocatable
	allocatable := make(map[string]string)
	for k, v := range node.Status.Allocatable {
		allocatable[string(k)] = v.String()
	}

	// System Info
	sysInfo := NodeSystemInfo{
		MachineID:               node.Status.NodeInfo.MachineID,
		SystemUUID:              node.Status.NodeInfo.SystemUUID,
		BootID:                  node.Status.NodeInfo.BootID,
		KernelVersion:           node.Status.NodeInfo.KernelVersion,
		OSImage:                 node.Status.NodeInfo.OSImage,
		ContainerRuntimeVersion: node.Status.NodeInfo.ContainerRuntimeVersion,
		KubeletVersion:          node.Status.NodeInfo.KubeletVersion,
		KubeProxyVersion:        node.Status.NodeInfo.KubeProxyVersion,
		OperatingSystem:         node.Status.NodeInfo.OperatingSystem,
		Architecture:            node.Status.NodeInfo.Architecture,
	}

	// Calculate percentages
	allocatableCPU := node.Status.Allocatable.Cpu().MilliValue()
	allocatableMem := node.Status.Allocatable.Memory().Value()
	cpuPercent := 0
	memPercent := 0
	if allocatableCPU > 0 {
		cpuPercent = int((cpuRequests * 100) / allocatableCPU)
	}
	if allocatableMem > 0 {
		memPercent = int((memRequests * 100) / allocatableMem)
	}

	resources := NodeResources{
		CPURequests:    fmt.Sprintf("%dm", cpuRequests),
		CPULimits:      fmt.Sprintf("%dm", cpuLimits),
		MemoryRequests: formatBytes(memRequests),
		MemoryLimits:   formatBytes(memLimits),
		CPUPercent:     cpuPercent,
		MemoryPercent:  memPercent,
	}

	detail := NodeDetail{
		NodeInfo:    toNodeInfo(node),
		Conditions:  conditions,
		PodCount:    podCount,
		Labels:      node.Labels,
		Taints:      taints,
		Addresses:   addresses,
		Capacity:    capacity,
		Allocatable: allocatable,
		SystemInfo:  sysInfo,
		PodCIDR:     node.Spec.PodCIDR,
		PodCIDRs:    node.Spec.PodCIDRs,
		Resources:   resources,
	}

	c.JSON(http.StatusOK, detail)
}

// formatBytes converts bytes to human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.0f%ci", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func toNodeInfo(node *corev1.Node) NodeInfo {
	status := "NotReady"
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			status = "Ready"
			break
		}
	}

	var internalIP string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			internalIP = addr.Address
			break
		}
	}

	// Roles logic (simplified)
	var roles []string
	for k := range node.Labels {
		if len(k) > 15 && k[:15] == "node-role.kubernetes.io/" {
			roles = append(roles, k[15:]) // Incorrect logic for label key prefix check but strictly speaking assumes standard labels
            // Better: strings.HasPrefix
		}
	}
    // Correcting roles logic to use label existence check
    roles = []string{}
    for k := range node.Labels {
        if len(k) >= 24 && k[:24] == "node-role.kubernetes.io/" {
             roles = append(roles, k[24:])
        } else if k == "node-role.kubernetes.io/master" || k == "node-role.kubernetes.io/control-plane" {
             // Handle empty value labels acting as flags
             // Actually, usually the key itself is the role name suffix or it's just a label presence
             // Let's keep it simple: if label key contains 'node-role', take the suffix
        }
    }
    
    // Simplest approach for role display
    if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
        roles = append(roles, "control-plane")
    }
    if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
        roles = append(roles, "master")
    }
    if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
        roles = append(roles, "worker")
    }
    if len(roles) == 0 {
        roles = []string{"<none>"}
    }
    

	return NodeInfo{
		Name:       node.Name,
		Status:     status,
		Roles:      roles,
		Version:    node.Status.NodeInfo.KubeletVersion,
		InternalIP: internalIP,
		CPU:        node.Status.Capacity.Cpu().String(),
		Memory:     node.Status.Capacity.Memory().String(),
		Age:        FormatAge(node.CreationTimestamp.Time),
	}
}

func toPodInfo(pod *corev1.Pod) PodInfo {
	readyContainers := 0
	totalContainers := len(pod.Spec.Containers)
	var restarts int32 = 0
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			readyContainers++
		}
		restarts += cs.RestartCount
	}

	return PodInfo{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Status:    string(pod.Status.Phase),
		Ready:     fmt.Sprintf("%d/%d", readyContainers, totalContainers),
		Restarts:  restarts,
		Age:       FormatAge(pod.CreationTimestamp.Time),
		Node:      pod.Spec.NodeName,
	}
}
