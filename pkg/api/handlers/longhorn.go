package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// longhornVolumeGVRs lists the GVRs we try in order. Newer Longhorn (v1.3+)
// serves v1beta2; older versions serve v1beta1.
var longhornVolumeGVRs = []schema.GroupVersionResource{
	{Group: "longhorn.io", Version: "v1beta2", Resource: "volumes"},
	{Group: "longhorn.io", Version: "v1beta1", Resource: "volumes"},
}

// GetClusterLonghornVolumes lists Longhorn Volume CRs across all namespaces.
// If Longhorn isn't installed, returns an empty list with available=false.
func GetClusterLonghornVolumes(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	dyn, err := dynamic.NewForConfig(client.RestConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build dynamic client", "details": err.Error()})
		return
	}

	var (
		items []unstructured.Unstructured
		used  schema.GroupVersionResource
	)
	for _, gvr := range longhornVolumeGVRs {
		list, err := dyn.Resource(gvr).List(c.Request.Context(), metav1.ListOptions{})
		if err == nil {
			items = list.Items
			used = gvr
			break
		}
		// Not installed / wrong version: try the next GVR.
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			continue
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed", "details": err.Error()})
		return
	}

	if used.Group == "" {
		c.JSON(http.StatusOK, gin.H{
			"items":     []any{},
			"available": false,
			"hint":      "Longhorn Volume CRD not found. Install Longhorn or check the cluster.",
		})
		return
	}

	rows := make([]gin.H, 0, len(items))
	for _, it := range items {
		spec, _, _ := unstructured.NestedMap(it.Object, "spec")
		status, _, _ := unstructured.NestedMap(it.Object, "status")
		rows = append(rows, gin.H{
			"name":             it.GetName(),
			"namespace":        it.GetNamespace(),
			"state":            getStr(status, "state"),
			"robustness":       getStr(status, "robustness"),
			"size":             humanBytes(getNumStr(spec, "size")),
			"sizeBytes":        getNumStr(spec, "size"),
			"actualSize":       humanBytes(getNumStr(status, "actualSize")),
			"actualSizeBytes":  getNumStr(status, "actualSize"),
			"numberOfReplicas": getInt(spec, "numberOfReplicas"),
			"frontend":         getStr(spec, "frontend"),
			"node":             firstNonEmpty(getStr(status, "currentNodeID"), getStr(status, "ownerID"), getStr(spec, "nodeID")),
			"pvcName":          getStr(status, "kubernetesStatus", "pvcName"),
			"pvcNamespace":     getStr(status, "kubernetesStatus", "namespace"),
			"age":              FormatAge(it.GetCreationTimestamp().Time),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     rows,
		"available": true,
		"version":   used.Version,
	})
}

// GetClusterLonghornVolumeDetail returns the raw unstructured Volume object
// (used by the detail drawer for full inspection).
func GetClusterLonghornVolumeDetail(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	dyn, err := dynamic.NewForConfig(client.RestConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build dynamic client", "details": err.Error()})
		return
	}

	for _, gvr := range longhornVolumeGVRs {
		obj, err := dyn.Resource(gvr).Namespace(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
		if err == nil {
			c.JSON(http.StatusOK, obj.Object)
			return
		}
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			continue
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get failed", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Longhorn Volume not found"})
}

// -------- small helpers --------

func getStr(m map[string]interface{}, path ...string) string {
	cur := m
	for i, p := range path {
		v, ok := cur[p]
		if !ok {
			return ""
		}
		if i == len(path)-1 {
			if s, ok := v.(string); ok {
				return s
			}
			return ""
		}
		cur, ok = v.(map[string]interface{})
		if !ok {
			return ""
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	case int:
		return int64(x)
	}
	return 0
}

// Longhorn stores byte sizes as strings ("10737418240"). Handle both.
func getNumStr(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatInt(int64(x), 10)
	}
	return ""
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

// humanBytes converts a byte count string into IEC notation: "10Gi", "512Mi".
func humanBytes(s string) string {
	if s == "" {
		return ""
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n == 0 {
		return s
	}
	const k = 1024
	if n < k {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(k), 0
	for x := n / k; x >= k; x /= k {
		div *= k
		exp++
	}
	units := []string{"Ki", "Mi", "Gi", "Ti", "Pi"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	val := float64(n) / float64(div)
	if val >= 100 {
		return fmt.Sprintf("%.0f%s", val, units[exp])
	}
	return fmt.Sprintf("%.1f%s", val, units[exp])
}
