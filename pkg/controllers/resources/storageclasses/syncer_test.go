package storageclasses

import (
	"testing"

	synccontext "github.com/spectrocloud/vcluster/pkg/controllers/syncer/context"
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer/translator"
	"github.com/spectrocloud/vcluster/pkg/util/translate"
	"gotest.tools/assert"

	generictesting "github.com/spectrocloud/vcluster/pkg/controllers/syncer/testing"
	"k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSync(t *testing.T) {
	vObjectMeta := metav1.ObjectMeta{
		Name: "testsc",
	}
	vObject := &v1.StorageClass{
		ObjectMeta:  vObjectMeta,
		Provisioner: "my-provisioner",
	}
	pObject := &v1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: translate.PhysicalNameClusterScoped(vObjectMeta.Name, generictesting.DefaultTestTargetNamespace),
			Labels: map[string]string{
				translate.MarkerLabel: translate.Suffix,
			},
			Annotations: map[string]string{
				translator.NameAnnotation: "testsc",
			},
		},
		Provisioner: "my-provisioner",
	}
	vObjectUpdated := &v1.StorageClass{
		ObjectMeta:  vObjectMeta,
		Provisioner: "my-provisioner",
		Parameters: map[string]string{
			"TEST": "TEST",
		},
	}
	pObjectUpdated := &v1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: translate.PhysicalNameClusterScoped(vObjectMeta.Name, generictesting.DefaultTestTargetNamespace),
			Labels: map[string]string{
				translate.MarkerLabel: translate.Suffix,
			},
			Annotations: map[string]string{
				translator.NameAnnotation: "testsc",
			},
		},
		Provisioner: "my-provisioner",
		Parameters: map[string]string{
			"TEST": "TEST",
		},
	}

	generictesting.RunTests(t, []*generictesting.SyncTest{
		{
			Name:                "Sync Down",
			InitialVirtualState: []runtime.Object{vObject},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				v1.SchemeGroupVersion.WithKind("StorageClass"): {vObject},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				v1.SchemeGroupVersion.WithKind("StorageClass"): {pObject},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*storageClassSyncer).SyncDown(syncCtx, vObject)
				assert.NilError(t, err)
			},
		},
		{
			Name:                 "Sync",
			InitialVirtualState:  []runtime.Object{vObjectUpdated},
			InitialPhysicalState: []runtime.Object{pObject},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				v1.SchemeGroupVersion.WithKind("StorageClass"): {vObjectUpdated},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				v1.SchemeGroupVersion.WithKind("StorageClass"): {pObjectUpdated},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*storageClassSyncer).Sync(syncCtx, pObject, vObjectUpdated)
				assert.NilError(t, err)
			},
		},
	})
}
