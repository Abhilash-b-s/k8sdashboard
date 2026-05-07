package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// EventInfo represents event information
type EventInfo struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	Type           string `json:"type"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Source         string `json:"source"`
	InvolvedObject string `json:"involvedObject"`
	Count          int32  `json:"count"`
	FirstSeen      string `json:"firstSeen"`
	LastSeen       string `json:"lastSeen"`
	Age            string `json:"age"`
}

// NetworkPolicyInfo represents network policy information
type NetworkPolicyInfo struct {
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	PodSelector string   `json:"podSelector"`
	Ingress     int      `json:"ingress"`
	Egress      int      `json:"egress"`
	PolicyTypes []string `json:"policyTypes"`
	Age         string   `json:"age"`
}

// RoleInfo represents role information
type RoleInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Rules     int    `json:"rules"`
	Age       string `json:"age"`
}

// RoleBindingInfo represents role binding information
type RoleBindingInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Role      string `json:"role"`
	Subjects  int    `json:"subjects"`
	Age       string `json:"age"`
}

// ServiceAccountInfo represents service account information
type ServiceAccountInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Secrets   int    `json:"secrets"`
	Age       string `json:"age"`
}

// HPAInfo represents horizontal pod autoscaler information
type HPAInfo struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	Reference       string `json:"reference"`
	Targets         string `json:"targets"`
	MinPods         int32  `json:"minPods"`
	MaxPods         int32  `json:"maxPods"`
	CurrentReplicas int32  `json:"currentReplicas"`
	DesiredReplicas int32  `json:"desiredReplicas"`
	Age             string `json:"age"`
}

// GetEvents returns list of events
func GetEvents(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	events, err := k8s.InformerFactory.Core().V1().Events().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list events"})
		return
	}

	var result []EventInfo
	for _, event := range events {
		if namespaceFilter != "" && event.Namespace != namespaceFilter {
			continue
		}

		source := event.Source.Component
		if event.Source.Host != "" {
			source = fmt.Sprintf("%s/%s", event.Source.Component, event.Source.Host)
		}

		involvedObj := fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name)

		result = append(result, EventInfo{
			Name:           event.Name,
			Namespace:      event.Namespace,
			Type:           event.Type,
			Reason:         event.Reason,
			Message:        event.Message,
			Source:         source,
			InvolvedObject: involvedObj,
			Count:          event.Count,
			FirstSeen:      FormatAge(event.FirstTimestamp.Time),
			LastSeen:       FormatAge(event.LastTimestamp.Time),
			Age:            FormatAge(event.CreationTimestamp.Time),
		})
	}

	// Sort by last seen time (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastSeen < result[j].LastSeen
	})

	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetNetworkPolicies returns list of network policies
func GetNetworkPolicies(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	policies, err := k8s.InformerFactory.Networking().V1().NetworkPolicies().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list network policies"})
		return
	}

	var result []NetworkPolicyInfo
	for _, np := range policies {
		if namespaceFilter != "" && np.Namespace != namespaceFilter {
			continue
		}

		podSelector := "-"
		if len(np.Spec.PodSelector.MatchLabels) > 0 {
			podSelector = fmt.Sprintf("%v", np.Spec.PodSelector.MatchLabels)
		}

		policyTypes := make([]string, 0)
		for _, pt := range np.Spec.PolicyTypes {
			policyTypes = append(policyTypes, string(pt))
		}

		result = append(result, NetworkPolicyInfo{
			Name:        np.Name,
			Namespace:   np.Namespace,
			PodSelector: podSelector,
			Ingress:     len(np.Spec.Ingress),
			Egress:      len(np.Spec.Egress),
			PolicyTypes: policyTypes,
			Age:         FormatAge(np.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetRoles returns list of roles
func GetRoles(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	roles, err := k8s.InformerFactory.Rbac().V1().Roles().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list roles"})
		return
	}

	var result []RoleInfo
	for _, role := range roles {
		if namespaceFilter != "" && role.Namespace != namespaceFilter {
			continue
		}

		result = append(result, RoleInfo{
			Name:      role.Name,
			Namespace: role.Namespace,
			Rules:     len(role.Rules),
			Age:       FormatAge(role.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetRoleBindings returns list of role bindings
func GetRoleBindings(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	bindings, err := k8s.InformerFactory.Rbac().V1().RoleBindings().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list role bindings"})
		return
	}

	var result []RoleBindingInfo
	for _, rb := range bindings {
		if namespaceFilter != "" && rb.Namespace != namespaceFilter {
			continue
		}

		result = append(result, RoleBindingInfo{
			Name:      rb.Name,
			Namespace: rb.Namespace,
			Role:      fmt.Sprintf("%s/%s", rb.RoleRef.Kind, rb.RoleRef.Name),
			Subjects:  len(rb.Subjects),
			Age:       FormatAge(rb.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetServiceAccounts returns list of service accounts
func GetServiceAccounts(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	sas, err := k8s.InformerFactory.Core().V1().ServiceAccounts().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list service accounts"})
		return
	}

	var result []ServiceAccountInfo
	for _, sa := range sas {
		if namespaceFilter != "" && sa.Namespace != namespaceFilter {
			continue
		}

		result = append(result, ServiceAccountInfo{
			Name:      sa.Name,
			Namespace: sa.Namespace,
			Secrets:   len(sa.Secrets),
			Age:       FormatAge(sa.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetHPAs returns list of horizontal pod autoscalers
func GetHPAs(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	hpas, err := k8s.InformerFactory.Autoscaling().V2().HorizontalPodAutoscalers().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list HPAs"})
		return
	}

	var result []HPAInfo
	for _, hpa := range hpas {
		if namespaceFilter != "" && hpa.Namespace != namespaceFilter {
			continue
		}

		reference := fmt.Sprintf("%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name)

		targets := "-"
		if len(hpa.Spec.Metrics) > 0 {
			targets = fmt.Sprintf("%d metrics", len(hpa.Spec.Metrics))
		}

		result = append(result, HPAInfo{
			Name:            hpa.Name,
			Namespace:       hpa.Namespace,
			Reference:       reference,
			Targets:         targets,
			MinPods:         *hpa.Spec.MinReplicas,
			MaxPods:         hpa.Spec.MaxReplicas,
			CurrentReplicas: hpa.Status.CurrentReplicas,
			DesiredReplicas: hpa.Status.DesiredReplicas,
			Age:             FormatAge(hpa.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetEventsForResource returns events for a specific resource
func GetEventsForResource(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	kind := c.Query("kind")

	events, err := k8s.Clientset.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s", name, kind),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list events"})
		return
	}

	var result []EventInfo
	for _, event := range events.Items {
		source := event.Source.Component
		if event.Source.Host != "" {
			source = fmt.Sprintf("%s/%s", event.Source.Component, event.Source.Host)
		}

		result = append(result, EventInfo{
			Name:           event.Name,
			Namespace:      event.Namespace,
			Type:           event.Type,
			Reason:         event.Reason,
			Message:        event.Message,
			Source:         source,
			InvolvedObject: fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
			Count:          event.Count,
			FirstSeen:      FormatAge(event.FirstTimestamp.Time),
			LastSeen:       FormatAge(event.LastTimestamp.Time),
			Age:            FormatAge(event.CreationTimestamp.Time),
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": result})
}
