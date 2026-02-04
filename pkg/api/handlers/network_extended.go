package handlers

import (
	"net/http"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/labels"
)

// GetIngressClasses returns list of ingress classes
func GetIngressClasses(c *gin.Context) {
	classes, err := k8s.InformerFactory.Networking().V1().IngressClasses().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list ingress classes"})
		return
	}

	var result []IngressClassInfo
	for _, ic := range classes {
		result = append(result, IngressClassInfo{
			Name:       ic.Name,
			Controller: ic.Spec.Controller,
			Age:        FormatAge(ic.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}
