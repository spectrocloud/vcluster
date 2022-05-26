package filters

import (
	"context"
	"net"
	"net/http"

	"github.com/pkg/errors"
	"github.com/spectrocloud/vcluster/pkg/constants"
	"github.com/spectrocloud/vcluster/pkg/controllers/resources/nodes/nodeservice"
	requestpkg "github.com/spectrocloud/vcluster/pkg/util/request"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type nodeName int

// nodeNameKey is the NodeName key for the context. It's of private type here. Because
// keys are interfaces and interfaces are equal when the type and the value is equal, this
// does not conflict with the keys defined in pkg/api.
const nodeNameKey nodeName = iota

func WithNodeName(h http.Handler, currentNamespace string, currentNamespaceClient client.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		nodeName, err := nodeNameFromHost(req.Context(), req.Host, currentNamespace, currentNamespaceClient)
		if err != nil {
			requestpkg.FailWithStatus(w, req, http.StatusInternalServerError, errors.Wrap(err, "find node name from host"))
			return
		} else if nodeName != "" {
			req = req.WithContext(context.WithValue(req.Context(), nodeNameKey, nodeName))
		}

		h.ServeHTTP(w, req)
	})
}

// NodeNameFrom returns a node name if there is any
func NodeNameFrom(ctx context.Context) (string, bool) {
	info, ok := ctx.Value(nodeNameKey).(string)
	return info, ok
}

func nodeNameFromHost(ctx context.Context, host, currentNamespace string, currentNamespaceClient client.Client) (string, error) {
	addr, err := net.ResolveUDPAddr("udp", host)
	if err == nil {
		clusterIP := addr.IP.String()
		serviceList := &corev1.ServiceList{}
		err = currentNamespaceClient.List(ctx, serviceList, client.InNamespace(currentNamespace), client.MatchingFields{constants.IndexByClusterIP: clusterIP})
		if err != nil {
			return "", err
		}

		// we found a service?
		if len(serviceList.Items) > 0 {
			serviceLabels := serviceList.Items[0].Labels
			if len(serviceLabels) > 0 && serviceLabels[nodeservice.ServiceNodeLabel] != "" {
				return serviceLabels[nodeservice.ServiceNodeLabel], nil
			}
		}
	}

	return "", nil
}
