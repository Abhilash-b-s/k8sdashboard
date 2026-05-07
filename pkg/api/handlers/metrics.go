package handlers

import (
	"context"
	"net/http"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetNodeMetrics returns node metrics
func GetNodeMetrics(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	if k8s.MetricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Metrics server not available"})
		return
	}

	metrics, err := k8s.MetricsClient.MetricsV1beta1().NodeMetricses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	nodes, _ := k8s.InformerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
	nodeMap := make(map[string]int64)
	memMap := make(map[string]int64)
	for _, n := range nodes {
		nodeMap[n.Name] = n.Status.Capacity.Cpu().MilliValue()
		memMap[n.Name] = n.Status.Capacity.Memory().Value()
	}

	result := make([]gin.H, 0)
	for _, m := range metrics.Items {
		cpuUsage := m.Usage.Cpu().MilliValue()
		memUsage := m.Usage.Memory().Value()
		cpuCapacity := nodeMap[m.Name]
		memCapacity := memMap[m.Name]

		cpuPercent := float64(0)
		if cpuCapacity > 0 {
			cpuPercent = float64(cpuUsage) / float64(cpuCapacity) * 100
		}
		memPercent := float64(0)
		if memCapacity > 0 {
			memPercent = float64(memUsage) / float64(memCapacity) * 100
		}

		result = append(result, gin.H{
			"name":       m.Name,
			"cpuUsage":   cpuUsage,
			"cpuPercent": cpuPercent,
			"memUsage":   memUsage,
			"memPercent": memPercent,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetPodMetrics returns pod metrics
func GetPodMetrics(c *gin.Context) {
	if k8s.MetricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Metrics server not available"})
		return
	}

	namespace := c.Query("namespace")
	var err error
	var metrics interface{}

	if namespace != "" {
		metrics, err = k8s.MetricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.Background(), metav1.ListOptions{})
	} else {
		metrics, err = k8s.MetricsClient.MetricsV1beta1().PodMetricses("").List(context.Background(), metav1.ListOptions{})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metrics)
}
