package handlers

import (
	"fmt"
	"net/http"
	"time"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/labels"
)

// GetDaemonSets returns list of daemonsets
func GetDaemonSets(c *gin.Context) {
	namespaceFilter := c.Query("namespace")
	dsets, err := k8s.InformerFactory.Apps().V1().DaemonSets().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list daemonsets"})
		return
	}

	var result []DaemonSetInfo
	for _, ds := range dsets {
		if namespaceFilter != "" && ds.Namespace != namespaceFilter {
			continue
		}
		result = append(result, DaemonSetInfo{
			Name:      ds.Name,
			Namespace: ds.Namespace,
			Desired:   ds.Status.DesiredNumberScheduled,
			Current:   ds.Status.CurrentNumberScheduled,
			Ready:     ds.Status.NumberReady,
			UpToDate:  ds.Status.UpdatedNumberScheduled,
			Available: ds.Status.NumberAvailable,
			Age:       FormatAge(ds.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetStatefulSets returns list of statefulsets
func GetStatefulSets(c *gin.Context) {
	namespaceFilter := c.Query("namespace")
	ssets, err := k8s.InformerFactory.Apps().V1().StatefulSets().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list statefulsets"})
		return
	}

	var result []StatefulSetInfo
	for _, ss := range ssets {
		if namespaceFilter != "" && ss.Namespace != namespaceFilter {
			continue
		}
		result = append(result, StatefulSetInfo{
			Name:      ss.Name,
			Namespace: ss.Namespace,
			Ready:     fmt.Sprintf("%d/%d", ss.Status.ReadyReplicas, *ss.Spec.Replicas),
			Age:       FormatAge(ss.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetReplicaSets returns list of replicasets
func GetReplicaSets(c *gin.Context) {
	namespaceFilter := c.Query("namespace")
	rsets, err := k8s.InformerFactory.Apps().V1().ReplicaSets().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list replicasets"})
		return
	}

	var result []ReplicaSetInfo
	for _, rs := range rsets {
		if namespaceFilter != "" && rs.Namespace != namespaceFilter {
			continue
		}
		
		desired := int32(0)
		if rs.Spec.Replicas != nil {
			desired = *rs.Spec.Replicas
		}

		result = append(result, ReplicaSetInfo{
			Name:      rs.Name,
			Namespace: rs.Namespace,
			Desired:   desired,
			Current:   rs.Status.Replicas,
			Ready:     rs.Status.ReadyReplicas,
			Age:       FormatAge(rs.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetReplicationControllers returns list of replication controllers
func GetReplicationControllers(c *gin.Context) {
	namespaceFilter := c.Query("namespace")
	rcs, err := k8s.InformerFactory.Core().V1().ReplicationControllers().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list replication controllers"})
		return
	}

	var result []ReplicationControllerInfo
	for _, rc := range rcs {
		if namespaceFilter != "" && rc.Namespace != namespaceFilter {
			continue
		}
		
		desired := int32(0)
		if rc.Spec.Replicas != nil {
			desired = *rc.Spec.Replicas
		}

		result = append(result, ReplicationControllerInfo{
			Name:      rc.Name,
			Namespace: rc.Namespace,
			Desired:   desired,
			Current:   rc.Status.Replicas,
			Ready:     rc.Status.ReadyReplicas,
			Age:       FormatAge(rc.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetJobs returns list of jobs
func GetJobs(c *gin.Context) {
	namespaceFilter := c.Query("namespace")
	jobs, err := k8s.InformerFactory.Batch().V1().Jobs().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list jobs"})
		return
	}

	var result []JobInfo
	for _, job := range jobs {
		if namespaceFilter != "" && job.Namespace != namespaceFilter {
			continue
		}
		
		completions := fmt.Sprintf("%d/%d", job.Status.Succeeded, *job.Spec.Completions)
		if job.Spec.Completions == nil {
			// If nil, it means 1 unless parallel execution is requested, but usually 1 success is needed
			// Simplified view for now
			completions = fmt.Sprintf("%d/1", job.Status.Succeeded)
		}

		duration := "-"
		if job.Status.StartTime != nil && job.Status.CompletionTime != nil {
			d := job.Status.CompletionTime.Time.Sub(job.Status.StartTime.Time)
			duration = d.String()
		} else if job.Status.StartTime != nil {
			duration = time.Since(job.Status.StartTime.Time).String()
		}

		result = append(result, JobInfo{
			Name:        job.Name,
			Namespace:   job.Namespace,
			Completions: completions,
			Duration:    duration,
			Age:         FormatAge(job.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetCronJobs returns list of cronjobs
func GetCronJobs(c *gin.Context) {
	namespaceFilter := c.Query("namespace")
	cronjobs, err := k8s.InformerFactory.Batch().V1().CronJobs().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list cronjobs"})
		return
	}

	var result []CronJobInfo
	for _, cj := range cronjobs {
		if namespaceFilter != "" && cj.Namespace != namespaceFilter {
			continue
		}

		lastSchedule := "-"
		if cj.Status.LastScheduleTime != nil {
			lastSchedule = FormatAge(cj.Status.LastScheduleTime.Time)
		}

		result = append(result, CronJobInfo{
			Name:         cj.Name,
			Namespace:    cj.Namespace,
			Schedule:     cj.Spec.Schedule,
			Suspend:      *cj.Spec.Suspend,
			Active:       len(cj.Status.Active),
			LastSchedule: lastSchedule,
			Age:          FormatAge(cj.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}
