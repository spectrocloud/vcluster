package priorityclasses

import (
	"github.com/spectrocloud/vcluster/pkg/constants"
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer"
	synccontext "github.com/spectrocloud/vcluster/pkg/controllers/syncer/context"
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer/translator"
	"github.com/spectrocloud/vcluster/pkg/util/translate"
	schedulingv1 "k8s.io/api/scheduling/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx *synccontext.RegisterContext) (syncer.Object, error) {
	return &priorityClassSyncer{
		Translator: translator.NewClusterTranslator(ctx, "priorityclass", &schedulingv1.PriorityClass{}, NewPriorityClassTranslator(ctx.Options.TargetNamespace)),
	}, nil
}

type priorityClassSyncer struct {
	translator.Translator
}

var _ syncer.IndicesRegisterer = &priorityClassSyncer{}

func (s *priorityClassSyncer) RegisterIndices(ctx *synccontext.RegisterContext) error {
	return ctx.VirtualManager.GetFieldIndexer().IndexField(ctx.Context, &schedulingv1.PriorityClass{}, constants.IndexByPhysicalName, func(rawObj client.Object) []string {
		return []string{translatePriorityClassName(ctx.Options.TargetNamespace, rawObj.GetName())}
	})
}

var _ syncer.Syncer = &priorityClassSyncer{}

func (s *priorityClassSyncer) SyncDown(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	newPriorityClass := s.translate(vObj.(*schedulingv1.PriorityClass))
	ctx.Log.Infof("create physical priority class %s", newPriorityClass.Name)
	err := ctx.PhysicalClient.Create(ctx.Context, newPriorityClass)
	if err != nil {
		ctx.Log.Infof("error syncing %s to physical cluster: %v", vObj.GetName(), err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (s *priorityClassSyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	// did the priority class change?
	updated := s.translateUpdate(pObj.(*schedulingv1.PriorityClass), vObj.(*schedulingv1.PriorityClass))
	if updated != nil {
		ctx.Log.Infof("updating physical priority class %s, because virtual priority class has changed", updated.Name)
		err := ctx.PhysicalClient.Update(ctx.Context, updated)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func NewPriorityClassTranslator(physicalNamespace string) translator.PhysicalNameTranslator {
	return func(vName string, vObj client.Object) string {
		return translatePriorityClassName(physicalNamespace, vName)
	}
}

func translatePriorityClassName(physicalNamespace, name string) string {
	// we have to prefix with vcluster as system is reserved
	return translate.PhysicalNameClusterScoped(name, physicalNamespace)
}
