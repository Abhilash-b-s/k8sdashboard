package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/labels"
)

// GetServices returns list of services
func GetServices(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	services, err := k8s.InformerFactory.Core().V1().Services().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list services"})
		return
	}

	var result []ServiceInfo
	for _, svc := range services {
		if namespaceFilter != "" && svc.Namespace != namespaceFilter {
			continue
		}

		var ports []string
		for _, p := range svc.Spec.Ports {
			ports = append(ports, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
		}

		result = append(result, ServiceInfo{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Type:      string(svc.Spec.Type),
			ClusterIP: svc.Spec.ClusterIP,
			Ports:     strings.Join(ports, ", "),
			Age:       FormatAge(svc.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetServiceDetail returns service details
func GetServiceDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")
	svc, err := k8s.InformerFactory.Core().V1().Services().Lister().Services(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}
	c.JSON(http.StatusOK, svc)
}

// GetIngresses returns list of ingresses
func GetIngresses(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	ingresses, err := k8s.InformerFactory.Networking().V1().Ingresses().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list ingresses"})
		return
	}

	var result []IngressInfo
	for _, ing := range ingresses {
		if namespaceFilter != "" && ing.Namespace != namespaceFilter {
			continue
		}

		var hosts []string
		for _, rule := range ing.Spec.Rules {
			hosts = append(hosts, rule.Host)
		}

		var lbIngress []string
		for _, lb := range ing.Status.LoadBalancer.Ingress {
			if lb.IP != "" {
				lbIngress = append(lbIngress, lb.IP)
			} else if lb.Hostname != "" {
				lbIngress = append(lbIngress, lb.Hostname)
			}
		}

		result = append(result, IngressInfo{
			Name:      ing.Name,
			Namespace: ing.Namespace,
			Hosts:     strings.Join(hosts, ", "),
			Address:   strings.Join(lbIngress, ", "),
			Age:       FormatAge(ing.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetConfigMaps returns list of configmaps
func GetConfigMaps(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	cms, err := k8s.InformerFactory.Core().V1().ConfigMaps().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list configmaps"})
		return
	}

	var result []ConfigMapInfo
	for _, cm := range cms {
		if namespaceFilter != "" && cm.Namespace != namespaceFilter {
			continue
		}

		result = append(result, ConfigMapInfo{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Keys:      len(cm.Data) + len(cm.BinaryData),
			Age:       FormatAge(cm.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetConfigMapDetail returns configmap details
func GetConfigMapDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")
	cm, err := k8s.InformerFactory.Core().V1().ConfigMaps().Lister().ConfigMaps(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ConfigMap not found"})
		return
	}
	c.JSON(http.StatusOK, cm)
}

// GetSecrets returns list of secrets
func GetSecrets(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	secrets, err := k8s.InformerFactory.Core().V1().Secrets().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list secrets"})
		return
	}

	var result []SecretInfo
	for _, secret := range secrets {
		if namespaceFilter != "" && secret.Namespace != namespaceFilter {
			continue
		}

		result = append(result, SecretInfo{
			Name:      secret.Name,
			Namespace: secret.Namespace,
			Type:      string(secret.Type),
			Keys:      len(secret.Data),
			Age:       FormatAge(secret.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetSecretDetail returns secret details
func GetSecretDetail(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")
	secret, err := k8s.InformerFactory.Core().V1().Secrets().Lister().Secrets(namespace).Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Secret not found"})
		return
	}
	// Sanitize secret data? For now return as is, usually dashboard allows viewing
	c.JSON(http.StatusOK, secret)
}
