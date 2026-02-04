package handlers

import (
	"net/http"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/labels"
)

// GetClusterRoles returns list of cluster roles
func GetClusterRoles(c *gin.Context) {
	roles, err := k8s.InformerFactory.Rbac().V1().ClusterRoles().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list cluster roles"})
		return
	}

	var result []ClusterRoleInfo
	for _, r := range roles {
		result = append(result, ClusterRoleInfo{
			Name:  r.Name,
			Rules: len(r.Rules),
			Age:   FormatAge(r.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetClusterRoleBindings returns list of cluster role bindings
func GetClusterRoleBindings(c *gin.Context) {
	bindings, err := k8s.InformerFactory.Rbac().V1().ClusterRoleBindings().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list cluster role bindings"})
		return
	}

	var result []ClusterRoleBindingInfo
	for _, b := range bindings {
		result = append(result, ClusterRoleBindingInfo{
			Name:     b.Name,
			Role:     b.RoleRef.Name,
			Subjects: len(b.Subjects),
			Age:      FormatAge(b.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}
