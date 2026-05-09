package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ============================================
// OVERVIEW HANDLERS
// ============================================

func GetClusterOverview(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	nodes, err := client.InformerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list nodes"})
		return
	}
	pods, err := client.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pods"})
		return
	}
	deploys, err := client.InformerFactory.Apps().V1().Deployments().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list deployments"})
		return
	}
	services, err := client.InformerFactory.Core().V1().Services().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list services"})
		return
	}

	readyNodes := 0
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNodes++
				break
			}
		}
	}

	runningPods, pendingPods, failedPods := 0, 0, 0
	namespaceSet := make(map[string]bool)
	for _, pod := range pods {
		namespaceSet[pod.Namespace] = true
		switch pod.Status.Phase {
		case corev1.PodRunning:
			runningPods++
		case corev1.PodPending:
			pendingPods++
		case corev1.PodFailed:
			failedPods++
		}
	}

	alloc := computeAllocationTotals(pods, nodes)

	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	response := OverviewResponse{
		TotalNodes:          len(nodes),
		ReadyNodes:          readyNodes,
		TotalPods:           len(pods),
		RunningPods:         runningPods,
		PendingPods:         pendingPods,
		FailedPods:          failedPods,
		TotalDeployments:    len(deploys),
		TotalServices:       len(services),
		Namespaces:          namespaces,
		CPURequestsMilli:    alloc.CPURequestsMilli,
		CPULimitsMilli:      alloc.CPULimitsMilli,
		CPUAllocatableMilli: alloc.CPUAllocatableMilli,
		MemRequestsBytes:    alloc.MemRequestsBytes,
		MemLimitsBytes:      alloc.MemLimitsBytes,
		MemAllocatableBytes: alloc.MemAllocatableBytes,
		PodCount:            alloc.PodCount,
		PodCapacity:         alloc.PodCapacity,
		CPURequestPct:       alloc.CPURequestPct,
		CPULimitPct:         alloc.CPULimitPct,
		MemRequestPct:       alloc.MemRequestPct,
		MemLimitPct:         alloc.MemLimitPct,
		PodPct:              alloc.PodPct,
	}

	c.JSON(http.StatusOK, response)
}

// allocationTotals captures cluster-wide request/limit/allocatable sums.
type allocationTotals struct {
	CPURequestsMilli, CPULimitsMilli, CPUAllocatableMilli int64
	MemRequestsBytes, MemLimitsBytes, MemAllocatableBytes int64
	PodCount, PodCapacity                                 int64
	CPURequestPct, CPULimitPct                            float64
	MemRequestPct, MemLimitPct, PodPct                    float64
}

// computeAllocationTotals sums pod requests/limits the same way `kubectl
// describe node` does: skip terminated pods, and for each running pod take
// max(any init container, sum of regular containers) per resource.
func computeAllocationTotals(pods []*corev1.Pod, nodes []*corev1.Node) allocationTotals {
	var t allocationTotals
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		t.PodCount++
		var rcpu, rmem, lcpu, lmem int64
		for _, ct := range pod.Spec.Containers {
			rcpu += ct.Resources.Requests.Cpu().MilliValue()
			rmem += ct.Resources.Requests.Memory().Value()
			lcpu += ct.Resources.Limits.Cpu().MilliValue()
			lmem += ct.Resources.Limits.Memory().Value()
		}
		var icpu, imem, ilcpu, ilmem int64
		for _, ct := range pod.Spec.InitContainers {
			if v := ct.Resources.Requests.Cpu().MilliValue(); v > icpu {
				icpu = v
			}
			if v := ct.Resources.Requests.Memory().Value(); v > imem {
				imem = v
			}
			if v := ct.Resources.Limits.Cpu().MilliValue(); v > ilcpu {
				ilcpu = v
			}
			if v := ct.Resources.Limits.Memory().Value(); v > ilmem {
				ilmem = v
			}
		}
		if icpu > rcpu {
			rcpu = icpu
		}
		if imem > rmem {
			rmem = imem
		}
		if ilcpu > lcpu {
			lcpu = ilcpu
		}
		if ilmem > lmem {
			lmem = ilmem
		}
		t.CPURequestsMilli += rcpu
		t.MemRequestsBytes += rmem
		t.CPULimitsMilli += lcpu
		t.MemLimitsBytes += lmem
	}
	for _, n := range nodes {
		t.CPUAllocatableMilli += n.Status.Allocatable.Cpu().MilliValue()
		t.MemAllocatableBytes += n.Status.Allocatable.Memory().Value()
		t.PodCapacity += n.Status.Allocatable.Pods().Value()
	}
	pct := func(num, den int64) float64 {
		if den == 0 {
			return 0
		}
		return float64(num) * 100 / float64(den)
	}
	t.CPURequestPct = pct(t.CPURequestsMilli, t.CPUAllocatableMilli)
	t.CPULimitPct = pct(t.CPULimitsMilli, t.CPUAllocatableMilli)
	t.MemRequestPct = pct(t.MemRequestsBytes, t.MemAllocatableBytes)
	t.MemLimitPct = pct(t.MemLimitsBytes, t.MemAllocatableBytes)
	t.PodPct = pct(t.PodCount, t.PodCapacity)
	return t
}

func GetClusterNamespaces(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespaces, err := client.Clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	nsList := make([]gin.H, 0, len(namespaces.Items))
	for _, ns := range namespaces.Items {
		nsList = append(nsList, gin.H{
			"name":   ns.Name,
			"status": string(ns.Status.Phase),
			"age":    FormatAge(ns.CreationTimestamp.Time),
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": nsList})
}

func GetClusterNamespaceDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	name := c.Param("name")
	ns, err := client.Clientset.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Namespace not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        ns.Name,
		"status":      string(ns.Status.Phase),
		"labels":      ns.Labels,
		"annotations": ns.Annotations,
		"age":         FormatAge(ns.CreationTimestamp.Time),
	})
}

// ============================================
// NODE HANDLERS
// ============================================

func GetClusterNodes(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	nodes, err := client.InformerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
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

func GetClusterNode(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	name := c.Param("name")
	node, err := client.InformerFactory.Core().V1().Nodes().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	pods, err := client.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
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

	var conditions []NodeCondition
	for _, cond := range node.Status.Conditions {
		conditions = append(conditions, NodeCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	var taints []NodeTaint
	for _, t := range node.Spec.Taints {
		taints = append(taints, NodeTaint{
			Key:    t.Key,
			Value:  t.Value,
			Effect: string(t.Effect),
		})
	}

	var addresses []NodeAddress
	for _, addr := range node.Status.Addresses {
		addresses = append(addresses, NodeAddress{
			Type:    string(addr.Type),
			Address: addr.Address,
		})
	}

	capacity := make(map[string]string)
	for k, v := range node.Status.Capacity {
		capacity[string(k)] = v.String()
	}

	allocatable := make(map[string]string)
	for k, v := range node.Status.Allocatable {
		allocatable[string(k)] = v.String()
	}

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

// ============================================
// POD HANDLERS
// ============================================

func GetClusterPods(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	pods, err := client.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pods"})
		return
	}

	var result []PodInfo
	for _, pod := range pods {
		if namespace == "" || pod.Namespace == namespace {
			result = append(result, toPodInfo(pod))
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterPodDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	pod, err := client.InformerFactory.Core().V1().Pods().Lister().Pods(namespace).Get(name)
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

		for _, port := range container.Ports {
			cd.Ports = append(cd.Ports, ContainerPort{
				Name:          port.Name,
				ContainerPort: port.ContainerPort,
				Protocol:      string(port.Protocol),
			})
		}

		for _, env := range container.Env {
			cd.Env = append(cd.Env, EnvVar{Name: env.Name, Value: "[hidden]"})
		}

		for _, vm := range container.VolumeMounts {
			cd.VolumeMounts = append(cd.VolumeMounts, VolumeMount{
				Name:      vm.Name,
				MountPath: vm.MountPath,
				ReadOnly:  vm.ReadOnly,
			})
		}

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

	ownerRefs := make([]OwnerReferenceInfo, 0)
	for _, ref := range pod.OwnerReferences {
		ownerRefs = append(ownerRefs, OwnerReferenceInfo{
			Kind: ref.Kind,
			Name: ref.Name,
			UID:  string(ref.UID),
		})
	}

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

func UpdateClusterPod(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	var updateReq struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	}
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pod, err := client.Clientset.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	if updateReq.Labels != nil {
		pod.Labels = updateReq.Labels
	}
	if updateReq.Annotations != nil {
		pod.Annotations = updateReq.Annotations
	}

	updated, err := client.Clientset.CoreV1().Pods(namespace).Update(context.Background(), pod, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod updated", "name": updated.Name})
}

func DeleteClusterPod(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	err := client.Clientset.CoreV1().Pods(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod deleted", "name": name})
}

// ============================================
// DEPLOYMENT HANDLERS
// ============================================

func GetClusterDeployments(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	deployments, err := client.InformerFactory.Apps().V1().Deployments().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list deployments"})
		return
	}

	var result []DeploymentInfo
	for _, d := range deployments {
		if namespace == "" || d.Namespace == namespace {
			result = append(result, DeploymentInfo{
				Name:      d.Name,
				Namespace: d.Namespace,
				Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, *d.Spec.Replicas),
				UpToDate:  d.Status.UpdatedReplicas,
				Available: d.Status.AvailableReplicas,
				Age:       FormatAge(d.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterDeploymentDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	deployment, err := client.InformerFactory.Apps().V1().Deployments().Lister().Deployments(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(http.StatusOK, deployment)
}

func UpdateClusterDeployment(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	var updateReq struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	}
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deployment, err := client.Clientset.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	if updateReq.Labels != nil {
		deployment.Labels = updateReq.Labels
	}
	if updateReq.Annotations != nil {
		deployment.Annotations = updateReq.Annotations
	}

	updated, err := client.Clientset.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deployment updated", "name": updated.Name})
}

func DeleteClusterDeployment(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	err := client.Clientset.AppsV1().Deployments(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deployment deleted", "name": name})
}

func ScaleClusterDeployment(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	var scaleReq struct {
		Replicas int32 `json:"replicas"`
	}
	if err := c.ShouldBindJSON(&scaleReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scale, err := client.Clientset.AppsV1().Deployments(namespace).GetScale(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	scale.Spec.Replicas = scaleReq.Replicas
	_, err = client.Clientset.AppsV1().Deployments(namespace).UpdateScale(context.Background(), name, scale, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deployment scaled", "replicas": scaleReq.Replicas})
}

func RestartClusterDeployment(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	deployment, err := client.Clientset.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = client.Clientset.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deployment restarted", "name": name})
}

// ============================================
// DAEMONSET HANDLERS
// ============================================

func GetClusterDaemonSets(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	daemonsets, err := client.InformerFactory.Apps().V1().DaemonSets().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list daemonsets"})
		return
	}

	var result []DaemonSetInfo
	for _, ds := range daemonsets {
		if namespace == "" || ds.Namespace == namespace {
			result = append(result, DaemonSetInfo{
				Name:      ds.Name,
				Namespace: ds.Namespace,
				Desired:   ds.Status.DesiredNumberScheduled,
				Current:   ds.Status.CurrentNumberScheduled,
				Ready:     ds.Status.NumberReady,
				UpToDate:  ds.Status.UpdatedNumberScheduled,
				Available: ds.Status.NumberAvailable,
				Age:       FormatAge(ds.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterDaemonSetDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	ds, err := client.InformerFactory.Apps().V1().DaemonSets().Lister().DaemonSets(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DaemonSet not found"})
		return
	}

	c.JSON(http.StatusOK, ds)
}

func DeleteClusterDaemonSet(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	err := client.Clientset.AppsV1().DaemonSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "DaemonSet deleted", "name": name})
}

// ============================================
// STATEFULSET HANDLERS
// ============================================

func GetClusterStatefulSets(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	statefulsets, err := client.InformerFactory.Apps().V1().StatefulSets().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list statefulsets"})
		return
	}

	var result []StatefulSetInfo
	for _, sts := range statefulsets {
		if namespace == "" || sts.Namespace == namespace {
			result = append(result, StatefulSetInfo{
				Name:      sts.Name,
				Namespace: sts.Namespace,
				Ready:     fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, *sts.Spec.Replicas),
				Age:       FormatAge(sts.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterStatefulSetDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	sts, err := client.InformerFactory.Apps().V1().StatefulSets().Lister().StatefulSets(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "StatefulSet not found"})
		return
	}

	c.JSON(http.StatusOK, sts)
}

func UpdateClusterStatefulSet(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	var updateReq struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	}
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sts, err := client.Clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "StatefulSet not found"})
		return
	}

	if updateReq.Labels != nil {
		sts.Labels = updateReq.Labels
	}
	if updateReq.Annotations != nil {
		sts.Annotations = updateReq.Annotations
	}

	updated, err := client.Clientset.AppsV1().StatefulSets(namespace).Update(context.Background(), sts, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "StatefulSet updated", "name": updated.Name})
}

func DeleteClusterStatefulSet(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	err := client.Clientset.AppsV1().StatefulSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "StatefulSet deleted", "name": name})
}

func ScaleClusterStatefulSet(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	var scaleReq struct {
		Replicas int32 `json:"replicas"`
	}
	if err := c.ShouldBindJSON(&scaleReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scale, err := client.Clientset.AppsV1().StatefulSets(namespace).GetScale(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "StatefulSet not found"})
		return
	}

	scale.Spec.Replicas = scaleReq.Replicas
	_, err = client.Clientset.AppsV1().StatefulSets(namespace).UpdateScale(context.Background(), name, scale, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "StatefulSet scaled", "replicas": scaleReq.Replicas})
}

// ============================================
// REPLICASET HANDLERS
// ============================================

func GetClusterReplicaSets(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	replicasets, err := client.InformerFactory.Apps().V1().ReplicaSets().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list replicasets"})
		return
	}

	var result []ReplicaSetInfo
	for _, rs := range replicasets {
		if namespace == "" || rs.Namespace == namespace {
			result = append(result, ReplicaSetInfo{
				Name:      rs.Name,
				Namespace: rs.Namespace,
				Desired:   *rs.Spec.Replicas,
				Current:   rs.Status.Replicas,
				Ready:     rs.Status.ReadyReplicas,
				Age:       FormatAge(rs.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterReplicaSetDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	rs, err := client.InformerFactory.Apps().V1().ReplicaSets().Lister().ReplicaSets(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ReplicaSet not found"})
		return
	}

	c.JSON(http.StatusOK, rs)
}

func DeleteClusterReplicaSet(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	err := client.Clientset.AppsV1().ReplicaSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ReplicaSet deleted", "name": name})
}

// ============================================
// JOB HANDLERS
// ============================================

func GetClusterJobs(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	jobs, err := client.InformerFactory.Batch().V1().Jobs().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list jobs"})
		return
	}

	var result []gin.H
	for _, job := range jobs {
		if namespace == "" || job.Namespace == namespace {
			completions := int32(1)
			if job.Spec.Completions != nil {
				completions = *job.Spec.Completions
			}
			result = append(result, gin.H{
				"name":        job.Name,
				"namespace":   job.Namespace,
				"completions": fmt.Sprintf("%d/%d", job.Status.Succeeded, completions),
				"succeeded":   job.Status.Succeeded,
				"failed":      job.Status.Failed,
				"age":         FormatAge(job.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterJobDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	job, err := client.InformerFactory.Batch().V1().Jobs().Lister().Jobs(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// ============================================
// CRONJOB HANDLERS
// ============================================

func GetClusterCronJobs(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	cronjobs, err := client.InformerFactory.Batch().V1().CronJobs().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list cronjobs"})
		return
	}

	var result []CronJobInfo
	for _, cj := range cronjobs {
		if namespace == "" || cj.Namespace == namespace {
			lastSchedule := ""
			if cj.Status.LastScheduleTime != nil {
				lastSchedule = FormatAge(cj.Status.LastScheduleTime.Time)
			}
			result = append(result, CronJobInfo{
				Name:         cj.Name,
				Namespace:    cj.Namespace,
				Schedule:     cj.Spec.Schedule,
				Suspend:      *cj.Spec.Suspend,
				Active:       len(cj.Status.Active),
				LastSchedule: lastSchedule,
				Age:          FormatAge(cj.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterCronJobDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	cj, err := client.InformerFactory.Batch().V1().CronJobs().Lister().CronJobs(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CronJob not found"})
		return
	}

	c.JSON(http.StatusOK, cj)
}

// ============================================
// SERVICE HANDLERS
// ============================================

func GetClusterServices(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	services, err := client.InformerFactory.Core().V1().Services().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list services"})
		return
	}

	var result []gin.H
	for _, svc := range services {
		if namespace == "" || svc.Namespace == namespace {
			var ports []string
			for _, p := range svc.Spec.Ports {
				ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
			}
			result = append(result, gin.H{
				"name":       svc.Name,
				"namespace":  svc.Namespace,
				"type":       string(svc.Spec.Type),
				"clusterIP":  svc.Spec.ClusterIP,
				"externalIP": getExternalIP(svc),
				"ports":      ports,
				"age":        FormatAge(svc.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterServiceDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, err := client.InformerFactory.Core().V1().Services().Lister().Services(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	c.JSON(http.StatusOK, svc)
}

// ============================================
// LOGS HANDLER
// ============================================

func StreamClusterLogs(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	podName := c.Param("pod")
	container := c.Query("container")
	follow := c.Query("follow") != "false"

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	tailLines := int64(100)
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: &tailLines,
	}
	if container != "" {
		opts.Container = container
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	req := client.Clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		c.SSEvent("error", fmt.Sprintf("Failed to get logs: %v", err))
		c.Writer.Flush()
		return
	}
	defer stream.Close()

	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line, err := reader.ReadString('\n')
		if line != "" {
			c.SSEvent("log", line)
			c.Writer.Flush()
		}
		if err != nil {
			if err == io.EOF {
				if !follow {
					return
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return
		}
	}
}

// ============================================
// METRICS HANDLERS
// ============================================

func GetClusterNodeMetrics(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	if client.MetricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Metrics not available"})
		return
	}

	metrics, err := client.MetricsClient.MetricsV1beta1().NodeMetricses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	nodes, _ := client.InformerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
	nodeCapCPU := make(map[string]int64)
	nodeCapMem := make(map[string]int64)
	for _, n := range nodes {
		nodeCapCPU[n.Name] = n.Status.Capacity.Cpu().MilliValue()
		nodeCapMem[n.Name] = n.Status.Capacity.Memory().Value()
	}

	var result []gin.H
	for _, m := range metrics.Items {
		cpuUsage := m.Usage.Cpu().MilliValue()
		memUsage := m.Usage.Memory().Value()
		cpuCapacity := nodeCapCPU[m.Name]
		memCapacity := nodeCapMem[m.Name]

		cpuPercent := float64(0)
		if cpuCapacity > 0 {
			cpuPercent = float64(cpuUsage) / float64(cpuCapacity) * 100
		}
		memPercent := float64(0)
		if memCapacity > 0 {
			memPercent = float64(memUsage) / float64(memCapacity) * 100
		}

		result = append(result, gin.H{
			"name":       m.Name,
			"cpuUsage":   cpuUsage,
			"cpuPercent": cpuPercent,
			"memUsage":   memUsage,
			"memPercent": memPercent,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterPodMetrics(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	if client.MetricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Metrics not available"})
		return
	}

	namespace := c.Query("namespace")
	var metrics interface{}
	var err error

	if namespace != "" {
		metrics, err = client.MetricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.Background(), metav1.ListOptions{})
	} else {
		metrics, err = client.MetricsClient.MetricsV1beta1().PodMetricses("").List(context.Background(), metav1.ListOptions{})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": metrics})
}

// Helper function
func getExternalIP(svc *corev1.Service) string {
	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		if svc.Status.LoadBalancer.Ingress[0].IP != "" {
			return svc.Status.LoadBalancer.Ingress[0].IP
		}
		return svc.Status.LoadBalancer.Ingress[0].Hostname
	}
	if len(svc.Spec.ExternalIPs) > 0 {
		return svc.Spec.ExternalIPs[0]
	}
	return "<none>"
}
