package legacy

import (
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer"
	synccontext "github.com/spectrocloud/vcluster/pkg/controllers/syncer/context"
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer/translator"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewSyncer(ctx *synccontext.RegisterContext) (syncer.Object, error) {
	return &ingressSyncer{
		NamespacedTranslator: translator.NewNamespacedTranslator(ctx, "ingress", &networkingv1beta1.Ingress{}),
	}, nil
}

type ingressSyncer struct {
	translator.NamespacedTranslator
}

var _ syncer.Syncer = &ingressSyncer{}

func (s *ingressSyncer) SyncDown(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	return s.SyncDownCreate(ctx, vObj, s.translate(vObj.(*networkingv1beta1.Ingress)))
}

func (s *ingressSyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	vIngress := vObj.(*networkingv1beta1.Ingress)
	pIngress := pObj.(*networkingv1beta1.Ingress)

	updated := s.translateUpdateBackwards(pObj.(*networkingv1beta1.Ingress), vObj.(*networkingv1beta1.Ingress))
	if updated != nil {
		ctx.Log.Infof("update virtual ingress %s/%s, because ingress class name is out of sync", vIngress.Namespace, vIngress.Name)
		err := ctx.VirtualClient.Update(ctx.Context, updated)
		if err != nil {
			return ctrl.Result{}, err
		}

		// we will requeue anyways
		return ctrl.Result{}, nil
	}

	if !equality.Semantic.DeepEqual(vIngress.Status, pIngress.Status) {
		newIngress := vIngress.DeepCopy()
		newIngress.Status = pIngress.Status
		ctx.Log.Infof("update virtual ingress %s/%s, because status is out of sync", vIngress.Namespace, vIngress.Name)
		err := ctx.VirtualClient.Status().Update(ctx.Context, newIngress)
		if err != nil {
			return ctrl.Result{}, err
		}

		// we will requeue anyways
		return ctrl.Result{}, nil
	}

	return s.SyncDownUpdate(ctx, vObj, s.translateUpdate(pIngress, vIngress))
}

func SecretNamesFromIngress(ingress *networkingv1beta1.Ingress) []string {
	secrets := []string{}
	for _, tls := range ingress.Spec.TLS {
		if tls.SecretName != "" {
			secrets = append(secrets, ingress.Namespace+"/"+tls.SecretName)
		}
	}
	return translator.UniqueSlice(secrets)
}
