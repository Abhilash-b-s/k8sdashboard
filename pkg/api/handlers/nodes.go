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
	name := c.Param("name")
	node, err := k8s.InformerFactory.Core().V1().Nodes().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	// Get pods on this node
	pods, err := k8s.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	var nodePods []PodInfo
	if err == nil {
		for _, pod := range pods {
			if pod.Spec.NodeName == name {
				nodePods = append(nodePods, toPodInfo(pod))
			}
		}
	}

	var conditions []NodeCondition
	for _, cond := range node.Status.Conditions {
		conditions = append(conditions, NodeCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	detail := NodeDetail{
		NodeInfo:   toNodeInfo(node),
		Conditions: conditions,
		Pods:       nodePods,
		Labels:     node.Labels,
	}

	c.JSON(http.StatusOK, detail)
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
