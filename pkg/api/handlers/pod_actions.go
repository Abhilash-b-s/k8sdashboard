package handlers

import (
	"context"
	"net/http"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeletePod terminates a pod
func DeletePod(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	podName := c.Param("name")

	err := k8s.Clientset.CoreV1().Pods(namespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"message": "Failed to delete pod",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Pod deleted successfully",
		"namespace": namespace,
		"pod":       podName,
	})
}

// DeleteDeployment deletes a deployment
func DeleteDeployment(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	err := k8s.Clientset.AppsV1().Deployments(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"message": "Failed to delete deployment",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Deployment deleted successfully",
		"namespace": namespace,
		"name":      name,
	})
}

// ScaleDeployment scales a deployment to the specified replicas
func ScaleDeployment(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req struct {
		Replicas int32 `json:"replicas"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	scale, err := k8s.Clientset.AppsV1().Deployments(namespace).GetScale(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	scale.Spec.Replicas = req.Replicas
	_, err = k8s.Clientset.AppsV1().Deployments(namespace).UpdateScale(context.Background(), name, scale, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Deployment scaled successfully",
		"replicas": req.Replicas,
	})
}

// RestartDeployment restarts a deployment by updating an annotation
func RestartDeployment(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	deploy, err := k8s.Clientset.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().Format("2006-01-02T15:04:05Z")

	_, err = k8s.Clientset.AppsV1().Deployments(namespace).Update(context.Background(), deploy, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deployment restart triggered"})
}

// DeleteDaemonSet deletes a daemonset
func DeleteDaemonSet(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	err := k8s.Clientset.AppsV1().DaemonSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"message": "Failed to delete daemonset",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "DaemonSet deleted successfully",
		"namespace": namespace,
		"name":      name,
	})
}

// DeleteReplicaSet deletes a replicaset
func DeleteReplicaSet(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	err := k8s.Clientset.AppsV1().ReplicaSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"message": "Failed to delete replicaset",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "ReplicaSet deleted successfully",
		"namespace": namespace,
		"name":      name,
	})
}

// DeleteStatefulSet deletes a statefulset
func DeleteStatefulSet(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	err := k8s.Clientset.AppsV1().StatefulSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"message": "Failed to delete statefulset",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "StatefulSet deleted successfully",
		"namespace": namespace,
		"name":      name,
	})
}

// ScaleStatefulSet scales a statefulset to the specified replicas
func ScaleStatefulSet(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req struct {
		Replicas int32 `json:"replicas"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	scale, err := k8s.Clientset.AppsV1().StatefulSets(namespace).GetScale(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	scale.Spec.Replicas = req.Replicas
	_, err = k8s.Clientset.AppsV1().StatefulSets(namespace).UpdateScale(context.Background(), name, scale, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "StatefulSet scaled successfully",
		"replicas": req.Replicas,
	})
}

// UpdateDeployment updates a deployment with the provided spec
func UpdateDeployment(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	// Get the existing deployment first
	deploy, err := k8s.Clientset.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found", "details": err.Error()})
		return
	}

	// Parse the update request
	var req DeploymentUpdateRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Apply updates
	if req.Replicas != nil {
		deploy.Spec.Replicas = req.Replicas
	}

	if req.Image != "" && len(deploy.Spec.Template.Spec.Containers) > 0 {
		deploy.Spec.Template.Spec.Containers[0].Image = req.Image
	}

	if req.Labels != nil {
		if deploy.Labels == nil {
			deploy.Labels = make(map[string]string)
		}
		for k, v := range req.Labels {
			deploy.Labels[k] = v
		}
	}

	if req.Annotations != nil {
		if deploy.Annotations == nil {
			deploy.Annotations = make(map[string]string)
		}
		for k, v := range req.Annotations {
			deploy.Annotations[k] = v
		}
	}

	// Update the deployment
	updated, err := k8s.Clientset.AppsV1().Deployments(namespace).Update(context.Background(), deploy, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update deployment", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Deployment updated successfully",
		"deployment": updated,
	})
}

// UpdateStatefulSet updates a statefulset with the provided spec
func UpdateStatefulSet(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	// Get the existing statefulset first
	sts, err := k8s.Clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "StatefulSet not found", "details": err.Error()})
		return
	}

	// Parse the update request
	var req StatefulSetUpdateRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Apply updates
	if req.Replicas != nil {
		sts.Spec.Replicas = req.Replicas
	}

	if req.Image != "" && len(sts.Spec.Template.Spec.Containers) > 0 {
		sts.Spec.Template.Spec.Containers[0].Image = req.Image
	}

	if req.Labels != nil {
		if sts.Labels == nil {
			sts.Labels = make(map[string]string)
		}
		for k, v := range req.Labels {
			sts.Labels[k] = v
		}
	}

	if req.Annotations != nil {
		if sts.Annotations == nil {
			sts.Annotations = make(map[string]string)
		}
		for k, v := range req.Annotations {
			sts.Annotations[k] = v
		}
	}

	// Update the statefulset
	updated, err := k8s.Clientset.AppsV1().StatefulSets(namespace).Update(context.Background(), sts, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update statefulset", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "StatefulSet updated successfully",
		"statefulset": updated,
	})
}

// UpdatePod updates a pod's labels and annotations (limited updates allowed for pods)
func UpdatePod(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	// Get the existing pod first
	pod, err := k8s.Clientset.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found", "details": err.Error()})
		return
	}

	// Parse the update request
	var req PodUpdateRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Apply updates (pods have limited update options - mainly labels and annotations)
	if req.Labels != nil {
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		for k, v := range req.Labels {
			pod.Labels[k] = v
		}
	}

	if req.Annotations != nil {
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		for k, v := range req.Annotations {
			pod.Annotations[k] = v
		}
	}

	// Update the pod
	updated, err := k8s.Clientset.CoreV1().Pods(namespace).Update(context.Background(), pod, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update pod", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Pod updated successfully",
		"pod":     updated,
	})
}

// Request types for updates
type DeploymentUpdateRequest struct {
	Replicas    *int32            `json:"replicas,omitempty"`
	Image       string            `json:"image,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type StatefulSetUpdateRequest struct {
	Replicas    *int32            `json:"replicas,omitempty"`
	Image       string            `json:"image,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type PodUpdateRequest struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}
