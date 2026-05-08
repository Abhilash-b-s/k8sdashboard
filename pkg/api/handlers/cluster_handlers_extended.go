package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ============================================
// INGRESS HANDLERS
// ============================================

func GetClusterIngresses(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	ingresses, err := client.InformerFactory.Networking().V1().Ingresses().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list ingresses"})
		return
	}

	var result []gin.H
	for _, ing := range ingresses {
		if namespace == "" || ing.Namespace == namespace {
			var hosts []string
			for _, rule := range ing.Spec.Rules {
				hosts = append(hosts, rule.Host)
			}
			result = append(result, gin.H{
				"name":      ing.Name,
				"namespace": ing.Namespace,
				"hosts":     hosts,
				"age":       FormatAge(ing.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterIngressDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	ing, err := client.InformerFactory.Networking().V1().Ingresses().Lister().Ingresses(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ingress not found"})
		return
	}

	c.JSON(http.StatusOK, ing)
}

func GetClusterIngressClasses(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	classes, err := client.InformerFactory.Networking().V1().IngressClasses().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list ingress classes"})
		return
	}

	var result []gin.H
	for _, ic := range classes {
		result = append(result, gin.H{
			"name":       ic.Name,
			"controller": ic.Spec.Controller,
			"age":        FormatAge(ic.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// ============================================
// NETWORK POLICY HANDLERS
// ============================================

func GetClusterNetworkPolicies(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	policies, err := client.InformerFactory.Networking().V1().NetworkPolicies().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list network policies"})
		return
	}

	var result []gin.H
	for _, np := range policies {
		if namespace == "" || np.Namespace == namespace {
			result = append(result, gin.H{
				"name":      np.Name,
				"namespace": np.Namespace,
				"age":       FormatAge(np.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterNetworkPolicyDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	np, err := client.InformerFactory.Networking().V1().NetworkPolicies().Lister().NetworkPolicies(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "NetworkPolicy not found"})
		return
	}

	c.JSON(http.StatusOK, np)
}

// ============================================
// CONFIGMAP HANDLERS
// ============================================

func GetClusterConfigMaps(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	configmaps, err := client.InformerFactory.Core().V1().ConfigMaps().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list configmaps"})
		return
	}

	var result []gin.H
	for _, cm := range configmaps {
		if namespace == "" || cm.Namespace == namespace {
			result = append(result, gin.H{
				"name":      cm.Name,
				"namespace": cm.Namespace,
				"keys":      len(cm.Data),
				"age":       FormatAge(cm.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterConfigMapDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	cm, err := client.InformerFactory.Core().V1().ConfigMaps().Lister().ConfigMaps(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ConfigMap not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        cm.Name,
		"namespace":   cm.Namespace,
		"labels":      cm.Labels,
		"annotations": cm.Annotations,
		"data":        cm.Data,
		"age":         FormatAge(cm.CreationTimestamp.Time),
	})
}

// ============================================
// SECRET HANDLERS
// ============================================

func GetClusterSecrets(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	secrets, err := client.InformerFactory.Core().V1().Secrets().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list secrets"})
		return
	}

	var result []gin.H
	for _, s := range secrets {
		if namespace == "" || s.Namespace == namespace {
			result = append(result, gin.H{
				"name":      s.Name,
				"namespace": s.Namespace,
				"type":      string(s.Type),
				"keys":      len(s.Data),
				"age":       FormatAge(s.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterSecretDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	secret, err := client.InformerFactory.Core().V1().Secrets().Lister().Secrets(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
		return
	}

	// Default response: keys with byte sizes (no values).
	// Pass ?reveal=true to include decoded values — gated explicitly so that
	// values aren't shipped to the browser unless the user clicked Reveal.
	reveal := c.Query("reveal") == "true"
	keys := make([]gin.H, 0, len(secret.Data))
	for k, v := range secret.Data {
		entry := gin.H{"key": k, "size": len(v)}
		if reveal {
			entry["value"] = string(v)
		}
		keys = append(keys, entry)
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        secret.Name,
		"namespace":   secret.Namespace,
		"type":        string(secret.Type),
		"labels":      secret.Labels,
		"annotations": secret.Annotations,
		"data":        keys,
		"keys":        keys, // legacy alias
		"age":         FormatAge(secret.CreationTimestamp.Time),
	})
}

// ============================================
// STORAGE HANDLERS
// ============================================

func GetClusterPersistentVolumes(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	pvs, err := client.InformerFactory.Core().V1().PersistentVolumes().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list persistent volumes"})
		return
	}

	var result []gin.H
	for _, pv := range pvs {
		claim := ""
		if pv.Spec.ClaimRef != nil {
			claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		}
		result = append(result, gin.H{
			"name":          pv.Name,
			"capacity":      pv.Spec.Capacity.Storage().String(),
			"reclaimPolicy": string(pv.Spec.PersistentVolumeReclaimPolicy),
			"status":        string(pv.Status.Phase),
			"claim":         claim,
			"storageClass":  pv.Spec.StorageClassName,
			"age":           FormatAge(pv.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterPVDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	name := c.Param("name")
	pv, err := client.InformerFactory.Core().V1().PersistentVolumes().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PersistentVolume not found"})
		return
	}

	c.JSON(http.StatusOK, pv)
}

func GetClusterPersistentVolumeClaims(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	pvcs, err := client.InformerFactory.Core().V1().PersistentVolumeClaims().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list PVCs"})
		return
	}

	var result []gin.H
	for _, pvc := range pvcs {
		if namespace == "" || pvc.Namespace == namespace {
			storageClass := ""
			if pvc.Spec.StorageClassName != nil {
				storageClass = *pvc.Spec.StorageClassName
			}
			result = append(result, gin.H{
				"name":         pvc.Name,
				"namespace":    pvc.Namespace,
				"status":       string(pvc.Status.Phase),
				"volume":       pvc.Spec.VolumeName,
				"capacity":     pvc.Status.Capacity.Storage().String(),
				"storageClass": storageClass,
				"age":          FormatAge(pvc.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterPVCDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	pvc, err := client.InformerFactory.Core().V1().PersistentVolumeClaims().Lister().PersistentVolumeClaims(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PVC not found"})
		return
	}

	c.JSON(http.StatusOK, pvc)
}

func GetClusterStorageClasses(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	scs, err := client.InformerFactory.Storage().V1().StorageClasses().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list storage classes"})
		return
	}

	var result []gin.H
	for _, sc := range scs {
		reclaimPolicy := ""
		if sc.ReclaimPolicy != nil {
			reclaimPolicy = string(*sc.ReclaimPolicy)
		}
		volumeBindingMode := ""
		if sc.VolumeBindingMode != nil {
			volumeBindingMode = string(*sc.VolumeBindingMode)
		}
		// A StorageClass is the cluster default if either the GA or beta
		// is-default-class annotation is set to "true".
		isDefault := sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" ||
			sc.Annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true"
		allowExpansion := false
		if sc.AllowVolumeExpansion != nil {
			allowExpansion = *sc.AllowVolumeExpansion
		}
		result = append(result, gin.H{
			"name":              sc.Name,
			"provisioner":       sc.Provisioner,
			"reclaimPolicy":     reclaimPolicy,
			"volumeBinding":     volumeBindingMode,
			"volumeBindingMode": volumeBindingMode, // keep old key for any callers
			"allowExpansion":    allowExpansion,
			"isDefault":         isDefault,
			"age":               FormatAge(sc.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterStorageClassDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	name := c.Param("name")
	sc, err := client.InformerFactory.Storage().V1().StorageClasses().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "StorageClass not found"})
		return
	}

	c.JSON(http.StatusOK, sc)
}

// ============================================
// RBAC HANDLERS (renamed to avoid conflicts)
// ============================================

func GetClusterClusterRoles(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	roles, err := client.InformerFactory.Rbac().V1().ClusterRoles().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list cluster roles"})
		return
	}

	var result []gin.H
	for _, r := range roles {
		result = append(result, gin.H{
			"name": r.Name,
			"age":  FormatAge(r.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterClusterRoleDetailView(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	name := c.Param("name")
	role, err := client.InformerFactory.Rbac().V1().ClusterRoles().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ClusterRole not found"})
		return
	}

	c.JSON(http.StatusOK, role)
}

func GetClusterClusterRoleBindings(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	bindings, err := client.InformerFactory.Rbac().V1().ClusterRoleBindings().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list cluster role bindings"})
		return
	}

	var result []gin.H
	for _, b := range bindings {
		result = append(result, gin.H{
			"name":    b.Name,
			"roleRef": b.RoleRef.Name,
			"age":     FormatAge(b.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterClusterRoleBindingDetailView(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	name := c.Param("name")
	binding, err := client.InformerFactory.Rbac().V1().ClusterRoleBindings().Lister().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ClusterRoleBinding not found"})
		return
	}

	c.JSON(http.StatusOK, binding)
}

func GetClusterRolesView(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	roles, err := client.InformerFactory.Rbac().V1().Roles().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list roles"})
		return
	}

	var result []gin.H
	for _, r := range roles {
		if namespace == "" || r.Namespace == namespace {
			result = append(result, gin.H{
				"name":      r.Name,
				"namespace": r.Namespace,
				"age":       FormatAge(r.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterRoleDetailView(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	role, err := client.InformerFactory.Rbac().V1().Roles().Lister().Roles(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}

	c.JSON(http.StatusOK, role)
}

func GetClusterRoleBindingsView(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	bindings, err := client.InformerFactory.Rbac().V1().RoleBindings().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list role bindings"})
		return
	}

	var result []gin.H
	for _, b := range bindings {
		if namespace == "" || b.Namespace == namespace {
			result = append(result, gin.H{
				"name":      b.Name,
				"namespace": b.Namespace,
				"roleRef":   b.RoleRef.Name,
				"age":       FormatAge(b.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterRoleBindingDetailView(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	binding, err := client.InformerFactory.Rbac().V1().RoleBindings().Lister().RoleBindings(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "RoleBinding not found"})
		return
	}

	c.JSON(http.StatusOK, binding)
}

// ============================================
// SERVICE ACCOUNT HANDLERS
// ============================================

func GetClusterServiceAccounts(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	sas, err := client.InformerFactory.Core().V1().ServiceAccounts().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list service accounts"})
		return
	}

	var result []gin.H
	for _, sa := range sas {
		if namespace == "" || sa.Namespace == namespace {
			result = append(result, gin.H{
				"name":      sa.Name,
				"namespace": sa.Namespace,
				"secrets":   len(sa.Secrets),
				"age":       FormatAge(sa.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterServiceAccountDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	sa, err := client.InformerFactory.Core().V1().ServiceAccounts().Lister().ServiceAccounts(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
		return
	}

	c.JSON(http.StatusOK, sa)
}

// ============================================
// HPA HANDLERS
// ============================================

func GetClusterHPAs(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	hpas, err := client.InformerFactory.Autoscaling().V2().HorizontalPodAutoscalers().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list HPAs"})
		return
	}

	var result []gin.H
	for _, hpa := range hpas {
		if namespace == "" || hpa.Namespace == namespace {
			minReplicas := int32(1)
			if hpa.Spec.MinReplicas != nil {
				minReplicas = *hpa.Spec.MinReplicas
			}
			result = append(result, gin.H{
				"name":        hpa.Name,
				"namespace":   hpa.Namespace,
				"reference":   fmt.Sprintf("%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name),
				"minReplicas": minReplicas,
				"maxReplicas": hpa.Spec.MaxReplicas,
				"replicas":    hpa.Status.CurrentReplicas,
				"age":         FormatAge(hpa.CreationTimestamp.Time),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterHPADetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	hpa, err := client.InformerFactory.Autoscaling().V2().HorizontalPodAutoscalers().Lister().HorizontalPodAutoscalers(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "HPA not found"})
		return
	}

	c.JSON(http.StatusOK, hpa)
}

// ============================================
// EVENT HANDLERS
// ============================================

func GetClusterEvents(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Query("namespace")
	events, err := client.Clientset.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{
		Limit: 100,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, e := range events.Items {
		result = append(result, gin.H{
			"type":      e.Type,
			"reason":    e.Reason,
			"message":   e.Message,
			"object":    fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			"namespace": e.Namespace,
			"count":     e.Count,
			"lastSeen":  FormatAge(e.LastTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func GetClusterEventsForResource(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	events, err := client.Clientset.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, e := range events.Items {
		result = append(result, gin.H{
			"type":      e.Type,
			"reason":    e.Reason,
			"message":   e.Message,
			"object":    fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			"namespace": e.Namespace,
			"count":     e.Count,
			"lastSeen":  FormatAge(e.LastTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// Helper function for access modes
func getAccessModes(modes []corev1.PersistentVolumeAccessMode) []string {
	result := make([]string, len(modes))
	for i, m := range modes {
		result[i] = string(m)
	}
	return result
}
