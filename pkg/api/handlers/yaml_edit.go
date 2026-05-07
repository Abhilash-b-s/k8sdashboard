package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

// kindToGVR maps the URL :kind segment to a Kubernetes GroupVersionResource.
// Only resources we explicitly support YAML editing for are listed.
var kindToGVR = map[string]schema.GroupVersionResource{
	"deployments":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"statefulsets": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"daemonsets":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"replicasets":  {Group: "apps", Version: "v1", Resource: "replicasets"},
	"services":     {Group: "", Version: "v1", Resource: "services"},
	"configmaps":   {Group: "", Version: "v1", Resource: "configmaps"},
	"secrets":      {Group: "", Version: "v1", Resource: "secrets"},
	"pods":         {Group: "", Version: "v1", Resource: "pods"},
	"ingresses":    {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
}

// stripServerFields removes fields the API server populates so that the
// YAML the user edits stays focused on intent, not state.
func stripServerFields(obj map[string]interface{}) {
	if md, ok := obj["metadata"].(map[string]interface{}); ok {
		for _, k := range []string{
			"resourceVersion", "uid", "selfLink", "generation",
			"creationTimestamp", "managedFields", "ownerReferences",
		} {
			delete(md, k)
		}
	}
	delete(obj, "status")
}

// GetClusterResourceYAML returns the named resource as YAML, with
// server-managed fields stripped (kubectl-edit style).
func GetClusterResourceYAML(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	kind := c.Param("kind")
	namespace := c.Param("namespace")
	name := c.Param("name")

	gvr, ok := kindToGVR[kind]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported kind", "kind": kind})
		return
	}

	dyn, err := dynamic.NewForConfig(client.RestConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build dynamic client", "details": err.Error()})
		return
	}

	obj, err := dyn.Resource(gvr).Namespace(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "get failed", "details": err.Error()})
		}
		return
	}

	m := obj.UnstructuredContent()
	stripServerFields(m)

	out, err := yaml.Marshal(m)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "yaml marshal failed", "details": err.Error()})
		return
	}

	c.Header("Content-Type", "application/yaml")
	c.String(http.StatusOK, string(out))
}

// UpdateClusterResourceYAML accepts a YAML body, re-attaches the current
// resourceVersion (to avoid lost-update), and PUTs the resource back.
func UpdateClusterResourceYAML(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	kind := c.Param("kind")
	namespace := c.Param("namespace")
	name := c.Param("name")

	gvr, ok := kindToGVR[kind]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported kind", "kind": kind})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body", "details": err.Error()})
		return
	}

	// Parse YAML into a generic map; sigs.k8s.io/yaml accepts both YAML and JSON.
	var m map[string]interface{}
	if err := yaml.Unmarshal(body, &m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid YAML", "details": err.Error()})
		return
	}

	if m == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty manifest"})
		return
	}

	dyn, err := dynamic.NewForConfig(client.RestConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build dynamic client", "details": err.Error()})
		return
	}

	// Get existing object purely to read its current resourceVersion.
	existing, err := dyn.Resource(gvr).Namespace(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "get failed", "details": err.Error()})
		}
		return
	}

	md, _ := m["metadata"].(map[string]interface{})
	if md == nil {
		md = map[string]interface{}{}
		m["metadata"] = md
	}
	md["resourceVersion"] = existing.GetResourceVersion()
	// Force name/namespace to match the URL — protects against renames-by-edit.
	md["name"] = name
	if existing.GetNamespace() != "" {
		md["namespace"] = namespace
	}

	updated := existing.DeepCopy()
	updated.SetUnstructuredContent(m)

	out, err := dyn.Resource(gvr).Namespace(namespace).Update(c.Request.Context(), updated, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsConflict(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "resource changed since you opened it; reload and re-edit", "details": err.Error()})
		} else if apierrors.IsInvalid(err) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid manifest", "details": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed", "details": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "resource updated",
		"resourceVersion": out.GetResourceVersion(),
	})
}
