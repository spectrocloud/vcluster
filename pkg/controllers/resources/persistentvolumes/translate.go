package persistentvolumes

import (
	synccontext "github.com/spectrocloud/vcluster/pkg/controllers/syncer/context"
	"github.com/spectrocloud/vcluster/pkg/util/translate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (s *persistentVolumeSyncer) translate(ctx *synccontext.SyncContext, vPv *corev1.PersistentVolume) *corev1.PersistentVolume {
	// translate the persistent volume
	pPV := s.TranslateMetadata(vPv).(*corev1.PersistentVolume)
	pPV.Spec.ClaimRef = nil
	pPV.Spec.StorageClassName = translateStorageClass(ctx.TargetNamespace, vPv.Spec.StorageClassName)

	// TODO: translate the storage secrets
	return pPV
}

func translateStorageClass(physicalNamespace, vStorageClassName string) string {
	if vStorageClassName == "" {
		return ""
	}
	return translate.PhysicalNameClusterScoped(vStorageClassName, physicalNamespace)
}

func (s *persistentVolumeSyncer) translateBackwards(pPv *corev1.PersistentVolume, vPvc *corev1.PersistentVolumeClaim) *corev1.PersistentVolume {
	// build virtual persistent volume
	vObj := pPv.DeepCopy()
	vObj.ResourceVersion = ""
	vObj.UID = ""
	vObj.ManagedFields = nil
	if vPvc != nil {
		vObj.Spec.ClaimRef.ResourceVersion = vPvc.ResourceVersion
		vObj.Spec.ClaimRef.UID = vPvc.UID
		vObj.Spec.ClaimRef.Name = vPvc.Name
		vObj.Spec.ClaimRef.Namespace = vPvc.Namespace
		if vPvc.Spec.StorageClassName != nil {
			vObj.Spec.StorageClassName = *vPvc.Spec.StorageClassName
		}
	}
	if vObj.Annotations == nil {
		vObj.Annotations = map[string]string{}
	}
	vObj.Annotations[HostClusterPersistentVolumeAnnotation] = pPv.Name
	return vObj
}

func (s *persistentVolumeSyncer) translateUpdateBackwards(ctx *synccontext.SyncContext, vPv *corev1.PersistentVolume, pPv *corev1.PersistentVolume, vPvc *corev1.PersistentVolumeClaim) *corev1.PersistentVolume {
	var updated *corev1.PersistentVolume

	// build virtual persistent volume
	translatedSpec := *pPv.Spec.DeepCopy()
	if vPvc != nil {
		translatedSpec.ClaimRef.ResourceVersion = vPvc.ResourceVersion
		translatedSpec.ClaimRef.UID = vPvc.UID
		translatedSpec.ClaimRef.Name = vPvc.Name
		translatedSpec.ClaimRef.Namespace = vPvc.Namespace
		if vPvc.Spec.StorageClassName != nil {
			translatedSpec.StorageClassName = *vPvc.Spec.StorageClassName
		}
	}

	// check storage class
	if !translate.IsManagedCluster(ctx.TargetNamespace, pPv) {
		if !equality.Semantic.DeepEqual(vPv.Spec.StorageClassName, translatedSpec.StorageClassName) {
			updated = newIfNil(updated, vPv)
			updated.Spec.StorageClassName = translatedSpec.StorageClassName
		}
	}

	// check claim ref
	if !equality.Semantic.DeepEqual(vPv.Spec.ClaimRef, translatedSpec.ClaimRef) {
		updated = newIfNil(updated, vPv)
		updated.Spec.ClaimRef = translatedSpec.ClaimRef
	}

	// check pv size
	if vPv.Annotations != nil && vPv.Annotations[HostClusterPersistentVolumeAnnotation] != "" && !equality.Semantic.DeepEqual(pPv.Spec.Capacity, vPv.Spec.Capacity) {
		updated = newIfNil(updated, vPv)
		updated.Spec.Capacity = translatedSpec.Capacity
	}

	return updated
}

func (s *persistentVolumeSyncer) translateUpdate(ctx *synccontext.SyncContext, vPv *corev1.PersistentVolume, pPv *corev1.PersistentVolume) *corev1.PersistentVolume {
	var updated *corev1.PersistentVolume

	// TODO: translate the storage secrets
	if !equality.Semantic.DeepEqual(pPv.Spec.PersistentVolumeSource, vPv.Spec.PersistentVolumeSource) {
		updated = newIfNil(updated, pPv)
		updated.Spec.PersistentVolumeSource = vPv.Spec.PersistentVolumeSource
	}

	if !equality.Semantic.DeepEqual(pPv.Spec.Capacity, vPv.Spec.Capacity) {
		updated = newIfNil(updated, pPv)
		updated.Spec.Capacity = vPv.Spec.Capacity
	}

	if !equality.Semantic.DeepEqual(pPv.Spec.AccessModes, vPv.Spec.AccessModes) {
		updated = newIfNil(updated, pPv)
		updated.Spec.AccessModes = vPv.Spec.AccessModes
	}

	if !equality.Semantic.DeepEqual(pPv.Spec.PersistentVolumeReclaimPolicy, vPv.Spec.PersistentVolumeReclaimPolicy) {
		updated = newIfNil(updated, pPv)
		updated.Spec.PersistentVolumeReclaimPolicy = vPv.Spec.PersistentVolumeReclaimPolicy
	}

	translatedStorageClassName := translateStorageClass(ctx.TargetNamespace, vPv.Spec.StorageClassName)
	if !equality.Semantic.DeepEqual(pPv.Spec.StorageClassName, translatedStorageClassName) {
		updated = newIfNil(updated, pPv)
		updated.Spec.StorageClassName = translatedStorageClassName
	}

	if !equality.Semantic.DeepEqual(pPv.Spec.NodeAffinity, vPv.Spec.NodeAffinity) {
		updated = newIfNil(updated, pPv)
		updated.Spec.NodeAffinity = vPv.Spec.NodeAffinity
	}

	if !equality.Semantic.DeepEqual(pPv.Spec.VolumeMode, vPv.Spec.VolumeMode) {
		updated = newIfNil(updated, pPv)
		updated.Spec.VolumeMode = vPv.Spec.VolumeMode
	}

	if !equality.Semantic.DeepEqual(pPv.Spec.MountOptions, vPv.Spec.MountOptions) {
		updated = newIfNil(updated, pPv)
		updated.Spec.MountOptions = vPv.Spec.MountOptions
	}

	// check labels & annotations
	changed, updatedAnnotations, updatedLabels := s.TranslateMetadataUpdate(vPv, pPv)
	if changed {
		updated = newIfNil(updated, pPv)
		updated.Annotations = updatedAnnotations
		updated.Labels = updatedLabels
	}

	return updated
}

func newIfNil(updated *corev1.PersistentVolume, obj *corev1.PersistentVolume) *corev1.PersistentVolume {
	if updated == nil {
		return obj.DeepCopy()
	}
	return updated
}
