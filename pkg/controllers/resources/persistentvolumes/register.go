package persistentvolumes

import (
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer"
	synccontext "github.com/spectrocloud/vcluster/pkg/controllers/syncer/context"
)

func New(ctx *synccontext.RegisterContext) (syncer.Object, error) {
	if !ctx.Controllers["persistentvolumes"] {
		return NewFakeSyncer(ctx)
	}

	return NewSyncer(ctx)
}
