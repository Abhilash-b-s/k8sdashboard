package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetClusterGenericResources lists any kind lookupKind resolves (including
// custom resources) using the API server's Table rendering — the same columns
// `kubectl get` prints — so these kinds need no per-kind handler or informer.
// Response: {"columns": [...], "items": [{"name", "namespace", "cells": [...]}]}.
func GetClusterGenericResources(c *gin.Context) {
	client := GetClusterClient(c)
	if client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cluster client not found"})
		return
	}

	entry, ok := lookupKind(c.Param("kind"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported kind", "kind": c.Param("kind")})
		return
	}
	gvr := entry.GVR
	isCRD := gvr.Resource == "customresourcedefinitions"

	path := "/apis/" + gvr.Group + "/" + gvr.Version
	if gvr.Group == "" {
		path = "/api/" + gvr.Version
	}
	if ns := c.Query("namespace"); ns != "" && !entry.ClusterScoped {
		path += "/namespaces/" + ns
	}
	path += "/" + gvr.Resource

	req := client.Clientset.Discovery().RESTClient().Get().AbsPath(path).
		SetHeader("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io")
	if isCRD {
		// Need spec.group/names/versions to link each CRD to its instances.
		req = req.Param("includeObject", "Object")
	}
	raw, err := req.DoRaw(c.Request.Context())
	if err != nil {
		status := http.StatusInternalServerError
		if apierrors.IsNotFound(err) {
			status = http.StatusNotFound
		} else if apierrors.IsForbidden(err) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": "list failed", "details": err.Error()})
		return
	}

	var table metav1.Table
	if err := json.Unmarshal(raw, &table); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected response", "details": err.Error()})
		return
	}

	// Priority 0 columns are what `kubectl get` shows without -o wide.
	columns := []string{}
	idx := []int{}
	for i, col := range table.ColumnDefinitions {
		if col.Priority == 0 {
			columns = append(columns, col.Name)
			idx = append(idx, i)
		}
	}

	items := make([]gin.H, 0, len(table.Rows))
	for _, row := range table.Rows {
		obj := unstructured.Unstructured{Object: map[string]interface{}{}}
		_ = json.Unmarshal(row.Object.Raw, &obj.Object)
		cells := make([]interface{}, len(idx))
		for j, i := range idx {
			if i < len(row.Cells) {
				cells[j] = row.Cells[i]
			}
		}
		item := gin.H{"name": obj.GetName(), "namespace": obj.GetNamespace(), "cells": cells}
		if isCRD {
			addCRDFields(item, obj.Object)
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"columns": columns, "items": items})
}

// addCRDFields adds what the UI needs to browse a CRD's instances. The version
// used is the storage version (always served), falling back to the first served one.
func addCRDFields(item gin.H, crd map[string]interface{}) {
	group, _, _ := unstructured.NestedString(crd, "spec", "group")
	plural, _, _ := unstructured.NestedString(crd, "spec", "names", "plural")
	kind, _, _ := unstructured.NestedString(crd, "spec", "names", "kind")
	scope, _, _ := unstructured.NestedString(crd, "spec", "scope")
	versions, _, _ := unstructured.NestedSlice(crd, "spec", "versions")

	version := ""
	for _, v := range versions {
		m, _ := v.(map[string]interface{})
		name, _ := m["name"].(string)
		served, _ := m["served"].(bool)
		storage, _ := m["storage"].(bool)
		if storage && served {
			version = name
			break
		}
		if served && version == "" {
			version = name
		}
	}

	item["group"] = group
	item["version"] = version
	item["kind"] = kind
	item["scope"] = scope
	item["crKind"] = crKind(schema.GroupVersionResource{Group: group, Version: version, Resource: plural}, scope == "Cluster")
}
