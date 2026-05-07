package handlers

import (
	"fmt"
	"net/http"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/labels"
)

// GetPods returns list of pods
func GetPods(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	pods, err := k8s.InformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pods"})
		return
	}

	var result []PodInfo
	for _, pod := range pods {
		if namespaceFilter != "" && pod.Namespace != namespaceFilter {
			continue
		}
		result = append(result, toPodInfo(pod))
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetPod returns pod details
func GetPod(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
    namespace := c.Param("namespace")
    name := c.Param("pod")

    pod, err := k8s.InformerFactory.Core().V1().Pods().Lister().Pods(namespace).Get(name)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
        return
    }
    
    // We reuse PodInfo but might want a more detailed struct if `showPodDetail` uses more fields.
    // Looking at index.html, it uses: status, node, ip, hostIP, containers (name, image, status), labels.
    // PodInfo only has simple fields.
    // I should return the raw Pod object or a detailed struct? 
    // The frontend code accesses `data.containers`, `data.labels`. 
    // I will return the full Pod object for details to be safe, or a generic map.
    // Actually, Gin can serialize the K8s object directly if I pass it.
    // But `pod` is a pointer to `corev1.Pod`.
    c.JSON(http.StatusOK, pod)
}


// GetDeploymentDetail returns deployment details
func GetDeploymentDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	deploy, err := k8s.InformerFactory.Apps().V1().Deployments().Lister().Deployments(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(http.StatusOK, deploy)
}

// GetDeployments returns list of deployments
func GetDeployments(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	deploys, err := k8s.InformerFactory.Apps().V1().Deployments().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list deployments"})
		return
	}

	var result []DeploymentInfo
	for _, deploy := range deploys {
		if namespaceFilter != "" && deploy.Namespace != namespaceFilter {
			continue
		}

		deployInfo := DeploymentInfo{
			Name:      deploy.Name,
			Namespace: deploy.Namespace,
			Ready:     fmt.Sprintf("%d/%d", deploy.Status.ReadyReplicas, *deploy.Spec.Replicas),
			UpToDate:  deploy.Status.UpdatedReplicas,
			Available: deploy.Status.AvailableReplicas,
			Age:       FormatAge(deploy.CreationTimestamp.Time),
		}
		result = append(result, deployInfo)
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}
