package nodes

import (
	"github.com/spectrocloud/vcluster/pkg/controllers/resources/nodes/nodeservice"
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer"
	synccontext "github.com/spectrocloud/vcluster/pkg/controllers/syncer/context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx *synccontext.RegisterContext) (syncer.Object, error) {
	uncachedVirtualClient, err := client.New(ctx.VirtualManager.GetConfig(), client.Options{
		Scheme: ctx.VirtualManager.GetScheme(),
		Mapper: ctx.VirtualManager.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}

	nodeService := nodeservice.NewNodeServiceProvider(ctx.Options.ServiceName, ctx.CurrentNamespace, ctx.CurrentNamespaceClient, ctx.VirtualManager.GetClient(), uncachedVirtualClient)
	if !ctx.Controllers["nodes"] {
		return NewFakeSyncer(ctx, nodeService)
	}

	return NewSyncer(ctx, nodeService)
}
