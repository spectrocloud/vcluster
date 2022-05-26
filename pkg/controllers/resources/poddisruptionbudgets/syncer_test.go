package poddisruptionbudgets

import (
	"testing"

	synccontext "github.com/spectrocloud/vcluster/pkg/controllers/syncer/context"
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer/translator"
	"gotest.tools/assert"

	generictesting "github.com/spectrocloud/vcluster/pkg/controllers/syncer/testing"
	"github.com/spectrocloud/vcluster/pkg/util/translate"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestSync(t *testing.T) {
	vObjectMeta := metav1.ObjectMeta{
		Name:            "testPDB",
		Namespace:       "default",
		ResourceVersion: generictesting.FakeClientResourceVersion,
	}
	pObjectMeta := metav1.ObjectMeta{
		Name:      translate.PhysicalName("testPDB", vObjectMeta.Namespace),
		Namespace: "test",
		Annotations: map[string]string{
			translator.NameAnnotation:      vObjectMeta.Name,
			translator.NamespaceAnnotation: vObjectMeta.Namespace,
		},
		Labels: map[string]string{
			translate.NamespaceLabel: vObjectMeta.Namespace,
			translate.MarkerLabel:    translate.Suffix,
		},
		ResourceVersion: generictesting.FakeClientResourceVersion,
	}

	vclusterPDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: vObjectMeta,
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: int32(10)},
		},
	}

	hostClusterSyncedPDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: pObjectMeta,
		Spec:       vclusterPDB.Spec,
	}

	vclusterUpdatedPDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: vclusterPDB.ObjectMeta,
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: int32(5)},
		},
	}

	hostClusterSyncedUpdatedPDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: hostClusterSyncedPDB.ObjectMeta,
		Spec:       vclusterUpdatedPDB.Spec,
	}

	vclusterUpdatedSelectorPDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: vclusterPDB.ObjectMeta,
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: int32(5)},
			Selector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
		},
	}

	hostClusterSyncedUpdatedSelectorPDB := &policyv1.PodDisruptionBudget{
		ObjectMeta: hostClusterSyncedPDB.ObjectMeta,
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: vclusterUpdatedSelectorPDB.Spec.MaxUnavailable,
			Selector:       translator.TranslateLabelSelector(vclusterUpdatedSelectorPDB.Spec.Selector),
		},
	}

	generictesting.RunTests(t, []*generictesting.SyncTest{
		{
			Name: "Create Host Cluster PodDisruptionBudget",
			InitialVirtualState: []runtime.Object{
				vclusterPDB.DeepCopy(),
			},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				policyv1.SchemeGroupVersion.WithKind("PodDisruptionBudget"): {vclusterPDB.DeepCopy()},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				policyv1.SchemeGroupVersion.WithKind("PodDisruptionBudget"): {hostClusterSyncedPDB.DeepCopy()},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*pdbSyncer).SyncDown(syncCtx, vclusterPDB)
				assert.NilError(t, err)
			},
		},
		{
			Name: "Update Host Cluster PodDisruptionBudget's Spec",
			InitialVirtualState: []runtime.Object{
				vclusterUpdatedPDB.DeepCopy(),
			},
			InitialPhysicalState: []runtime.Object{
				hostClusterSyncedPDB.DeepCopy(),
			},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				policyv1.SchemeGroupVersion.WithKind("PodDisruptionBudget"): {vclusterUpdatedPDB.DeepCopy()},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				policyv1.SchemeGroupVersion.WithKind("PodDisruptionBudget"): {hostClusterSyncedUpdatedPDB.DeepCopy()},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*pdbSyncer).Sync(syncCtx, hostClusterSyncedPDB, vclusterUpdatedPDB)
				assert.NilError(t, err)
			},
		},
		{
			Name: "Update Host Cluster PodDisruptionBudget's Selector",
			InitialVirtualState: []runtime.Object{
				vclusterUpdatedSelectorPDB.DeepCopy(),
			},
			InitialPhysicalState: []runtime.Object{
				hostClusterSyncedPDB.DeepCopy(),
			},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				policyv1.SchemeGroupVersion.WithKind("PodDisruptionBudget"): {vclusterUpdatedSelectorPDB.DeepCopy()},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				policyv1.SchemeGroupVersion.WithKind("PodDisruptionBudget"): {hostClusterSyncedUpdatedSelectorPDB.DeepCopy()},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*pdbSyncer).Sync(syncCtx, hostClusterSyncedPDB, vclusterUpdatedSelectorPDB)
				assert.NilError(t, err)
			},
		},
	})
}
