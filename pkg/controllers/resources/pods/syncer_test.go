package pods

import (
	"testing"

	podtranslate "github.com/spectrocloud/vcluster/pkg/controllers/resources/pods/translate"
	synccontext "github.com/spectrocloud/vcluster/pkg/controllers/syncer/context"
	generictesting "github.com/spectrocloud/vcluster/pkg/controllers/syncer/testing"
	"github.com/spectrocloud/vcluster/pkg/controllers/syncer/translator"
	"github.com/spectrocloud/vcluster/pkg/util/translate"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"
)

func TestSync(t *testing.T) {
	pVclusterService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generictesting.DefaultTestVclusterServiceName,
			Namespace: generictesting.DefaultTestCurrentNamespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "1.2.3.4",
		},
	}
	translate.Suffix = generictesting.DefaultTestVclusterName
	pDNSService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      translate.PhysicalName("kube-dns", "kube-system"),
			Namespace: generictesting.DefaultTestTargetNamespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "2.2.2.2",
		},
	}
	vNamespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testns",
		},
	}
	vObjectMeta := metav1.ObjectMeta{
		Name:      "testpod",
		Namespace: vNamespace.Name,
	}
	pObjectMeta := metav1.ObjectMeta{
		Name:      translate.PhysicalName("testpod", "testns"),
		Namespace: "test",
		Annotations: map[string]string{
			podtranslate.ClusterAutoScalerAnnotation:  "false",
			podtranslate.LabelsAnnotation:             "",
			podtranslate.NameAnnotation:               vObjectMeta.Name,
			podtranslate.NamespaceAnnotation:          vObjectMeta.Namespace,
			translator.NameAnnotation:                 vObjectMeta.Name,
			translator.NamespaceAnnotation:            vObjectMeta.Namespace,
			podtranslate.ServiceAccountNameAnnotation: "",
			podtranslate.UIDAnnotation:                string(vObjectMeta.UID),
		},
		Labels: map[string]string{
			translate.NamespaceLabel: vObjectMeta.Namespace,
			translate.MarkerLabel:    translate.Suffix,
		},
	}
	pPodBase := &corev1.Pod{
		ObjectMeta: pObjectMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: pointer.Bool(false),
			EnableServiceLinks:           pointer.Bool(false),
			HostAliases: []corev1.HostAlias{{
				IP:        pVclusterService.Spec.ClusterIP,
				Hostnames: []string{"kubernetes", "kubernetes.default", "kubernetes.default.svc"},
			}},
			Hostname: vObjectMeta.Name,
		},
	}
	vPodWithNodeName := &corev1.Pod{
		ObjectMeta: vObjectMeta,
		Spec: corev1.PodSpec{
			NodeName: "test123",
		},
	}
	pPodWithNodeName := pPodBase.DeepCopy()
	pPodWithNodeName.Spec.NodeName = "test456"

	vPodWithNodeSelector := &corev1.Pod{
		ObjectMeta: vObjectMeta,
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"labelA": "valueA",
				"labelB": "valueB",
			},
		},
	}
	nodeSelectorOption := "labelB=enforcedB,otherLabel=abc"
	pPodWithNodeSelector := pPodBase.DeepCopy()
	pPodWithNodeSelector.Spec.NodeSelector = map[string]string{
		"labelA":     "valueA",
		"labelB":     "enforcedB",
		"otherLabel": "abc",
	}

	// pod security standards test objects
	vPodPSS := &corev1.Pod{
		ObjectMeta: vObjectMeta,
	}

	pPodPss := pPodBase.DeepCopy()

	vPodPSSR := &corev1.Pod{
		ObjectMeta: vObjectMeta,
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test-container",
					Ports: []corev1.ContainerPort{
						{HostPort: 80},
					},
				},
			},
		},
	}

	generictesting.RunTests(t, []*generictesting.SyncTest{
		{
			Name:                 "Delete virtual pod",
			InitialVirtualState:  []runtime.Object{vPodWithNodeName.DeepCopy()},
			InitialPhysicalState: []runtime.Object{pPodWithNodeName},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {
					pPodWithNodeName,
				},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*podSyncer).Sync(syncCtx, pPodWithNodeName.DeepCopy(), vPodWithNodeName)
				assert.NilError(t, err)
			},
		},
		{
			Name:                 "Sync and enforce NodeSelector",
			InitialVirtualState:  []runtime.Object{vPodWithNodeSelector.DeepCopy(), vNamespace.DeepCopy()},
			InitialPhysicalState: []runtime.Object{pVclusterService.DeepCopy(), pDNSService.DeepCopy()},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {vPodWithNodeSelector.DeepCopy()},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {
					pPodWithNodeSelector,
				},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				ctx.Options.EnforceNodeSelector = true
				ctx.Options.NodeSelector = nodeSelectorOption
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*podSyncer).SyncDown(syncCtx, vPodWithNodeSelector.DeepCopy())
				assert.NilError(t, err)
			},
		},
		{
			Name:                 "SyncDown pods without any pod security standards",
			InitialVirtualState:  []runtime.Object{vPodPSS.DeepCopy(), vNamespace.DeepCopy()},
			InitialPhysicalState: []runtime.Object{pVclusterService.DeepCopy(), pDNSService.DeepCopy()},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {vPodPSS.DeepCopy()},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {pPodPss.DeepCopy()},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*podSyncer).SyncDown(syncCtx, vPodPSS.DeepCopy())
				assert.NilError(t, err)
			},
		},
		{
			Name:                 "Enforce privileged pod security standard",
			InitialVirtualState:  []runtime.Object{vPodPSS.DeepCopy(), vNamespace.DeepCopy()},
			InitialPhysicalState: []runtime.Object{pVclusterService.DeepCopy(), pDNSService.DeepCopy()},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {vPodPSS.DeepCopy()},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {pPodPss.DeepCopy()},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				ctx.Options.EnforcePodSecurityStandard = string(api.LevelPrivileged)
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*podSyncer).SyncDown(syncCtx, vPodPSS.DeepCopy())
				assert.NilError(t, err)
			},
		},
		{
			Name:                 "Enforce restricted pod security standard",
			InitialVirtualState:  []runtime.Object{vPodPSSR.DeepCopy(), vNamespace.DeepCopy()},
			InitialPhysicalState: []runtime.Object{pVclusterService.DeepCopy(), pDNSService.DeepCopy()},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {vPodPSSR.DeepCopy()},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Pod"): {},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				ctx.Options.EnforcePodSecurityStandard = string(api.LevelRestricted)
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*podSyncer).SyncDown(syncCtx, vPodPSSR.DeepCopy())
				assert.NilError(t, err)
			},
		},
	})
}
