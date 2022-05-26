package storageclasses

import (
	"github.com/spectrocloud/vcluster/pkg/constants"
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer"
	synccontext "github.com/spectrocloud/vcluster/pkg/controllers/syncer/context"
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer/translator"
	"github.com/spectrocloud/vcluster/pkg/util/translate"
	storagev1 "k8s.io/api/storage/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	DefaultStorageClassAnnotation = "storageclass.kubernetes.io/is-default-class"
)

func New(ctx *synccontext.RegisterContext) (syncer.Object, error) {
	return &storageClassSyncer{
		Translator: translator.NewClusterTranslator(ctx, "storageclass", &storagev1.StorageClass{}, NewStorageClassTranslator(ctx.Options.TargetNamespace), DefaultStorageClassAnnotation),
	}, nil
}

type storageClassSyncer struct {
	translator.Translator
}

var _ syncer.IndicesRegisterer = &storageClassSyncer{}

func (s *storageClassSyncer) RegisterIndices(ctx *synccontext.RegisterContext) error {
	return ctx.VirtualManager.GetFieldIndexer().IndexField(ctx.Context, &storagev1.StorageClass{}, constants.IndexByPhysicalName, func(rawObj client.Object) []string {
		return []string{translateStorageClassName(ctx.Options.TargetNamespace, rawObj.GetName())}
	})
}

var _ syncer.Syncer = &storageClassSyncer{}

func (s *storageClassSyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	// did the storage class change?
	updated := s.translateUpdate(pObj.(*storagev1.StorageClass), vObj.(*storagev1.StorageClass))
	if updated != nil {
		ctx.Log.Infof("updating physical storage class %s, because virtual storage class has changed", updated.Name)
		err := ctx.PhysicalClient.Update(ctx.Context, updated)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (s *storageClassSyncer) SyncDown(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	newStorageClass := s.translate(vObj.(*storagev1.StorageClass))
	ctx.Log.Infof("create physical storage class %s", newStorageClass.Name)
	err := ctx.PhysicalClient.Create(ctx.Context, newStorageClass)
	if err != nil {
		ctx.Log.Infof("error syncing %s to physical cluster: %v", vObj.GetName(), err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func NewStorageClassTranslator(physicalNamespace string) translator.PhysicalNameTranslator {
	return func(vName string, vObj client.Object) string {
		return translateStorageClassName(physicalNamespace, vName)
	}
}

func translateStorageClassName(physicalNamespace, name string) string {
	// we have to prefix with vcluster as system is reserved
	return translate.PhysicalNameClusterScoped(name, physicalNamespace)
}
