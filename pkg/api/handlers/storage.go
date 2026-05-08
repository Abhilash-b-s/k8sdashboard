package handlers

import (
	"fmt"
	"net/http"
	"k8s-dashboard/pkg/k8s"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetPersistentVolumeClaims returns list of PVCs
func GetPersistentVolumeClaims(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	namespaceFilter := c.Query("namespace")
	pvcs, err := k8s.InformerFactory.Core().V1().PersistentVolumeClaims().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pvcs"})
		return
	}

	var result []PVCInfo
	for _, pvc := range pvcs {
		if namespaceFilter != "" && pvc.Namespace != namespaceFilter {
			continue
		}

		capacity := "-"
		if cap, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capacity = cap.String()
		}

		storageClass := "-"
		if pvc.Spec.StorageClassName != nil {
			storageClass = *pvc.Spec.StorageClassName
		}

		result = append(result, PVCInfo{
			Name:         pvc.Name,
			Namespace:    pvc.Namespace,
			Status:       string(pvc.Status.Phase),
			Volume:       pvc.Spec.VolumeName,
			Capacity:     capacity,
			StorageClass: storageClass,
			Age:          FormatAge(pvc.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetPersistentVolumes returns list of PVs
func GetPersistentVolumes(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	pvs, err := k8s.InformerFactory.Core().V1().PersistentVolumes().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pvs"})
		return
	}

	var result []PVInfo
	for _, pv := range pvs {
		capacity := "-"
		if cap, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = cap.String()
		}

		claim := ""
		if pv.Spec.ClaimRef != nil {
			claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		}

		result = append(result, PVInfo{
			Name:          pv.Name,
			Capacity:      capacity,
			ReclaimPolicy: string(pv.Spec.PersistentVolumeReclaimPolicy),
			Status:        string(pv.Status.Phase),
			Claim:         claim,
			StorageClass:  pv.Spec.StorageClassName,
			Age:           FormatAge(pv.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetStorageClasses returns list of storage classes
func GetStorageClasses(c *gin.Context) {
	if !checkLegacyClientAvailable(c) {
		return
	}
	scs, err := k8s.InformerFactory.Storage().V1().StorageClasses().Lister().List(labels.Everything())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list storage classes"})
		return
	}

	var result []StorageClassInfo
	for _, sc := range scs {
		isDefault := sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" ||
			sc.Annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true"

		allowExpansion := false
		if sc.AllowVolumeExpansion != nil {
			allowExpansion = *sc.AllowVolumeExpansion
		}
		
		volumeBinding := "-"
		if sc.VolumeBindingMode != nil {
			volumeBinding = string(*sc.VolumeBindingMode)
		}

		result = append(result, StorageClassInfo{
			Name:           sc.Name,
			Provisioner:    sc.Provisioner,
			ReclaimPolicy:  string(*sc.ReclaimPolicy),
			VolumeBinding:  volumeBinding,
			AllowExpansion: allowExpansion,
			IsDefault:      isDefault,
			Age:            FormatAge(sc.CreationTimestamp.Time),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}
