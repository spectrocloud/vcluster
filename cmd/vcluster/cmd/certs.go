package cmd

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spectrocloud/vcluster/pkg/certs"
	"github.com/spectrocloud/vcluster/pkg/util/clienthelper"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
)

// CertsCmd holds the certs flags
type CertsCmd struct {
	Prefix        string
	ServiceCIDR   string
	ClusterDomain string
	ClusterName   string
	Namespace     string

	CertificateDir string
	EtcdReplicas   int
}

func NewCertsCommand() *cobra.Command {
	options := &CertsCmd{}
	cmd := &cobra.Command{
		Use:   "certs",
		Short: "Generates control plane certificates",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return ExecuteCerts(options)
		},
	}

	cmd.Flags().StringVar(&options.ClusterName, "cluster-name", "kubernetes", "The cluster name")
	cmd.Flags().StringVar(&options.ClusterDomain, "cluster-domain", "cluster.local", "The cluster domain ending that should be used for the virtual cluster")
	cmd.Flags().StringVar(&options.ServiceCIDR, "service-cidr", "10.96.0.0/12", "Service CIDR is the subnet used by k8s services")
	cmd.Flags().StringVar(&options.Prefix, "prefix", "vcluster", "Release name and prefix for generating the assets")
	cmd.Flags().StringVar(&options.Namespace, "namespace", "", "Namespace where to deploy the cert secret to")
	cmd.Flags().StringVar(&options.CertificateDir, "certificate-dir", "certs", "The temporary directory where the certificates will be stored")
	cmd.Flags().IntVar(&options.EtcdReplicas, "etcd-replicas", 1, "The etcd cluster size")
	return cmd
}

// write needed files to secret
var certMap = map[string]string{
	certs.AdminKubeConfigFileName:             certs.AdminKubeConfigFileName,
	certs.ControllerManagerKubeConfigFileName: certs.ControllerManagerKubeConfigFileName,
	certs.SchedulerKubeConfigFileName:         certs.SchedulerKubeConfigFileName,

	certs.APIServerCertName: certs.APIServerCertName,
	certs.APIServerKeyName:  certs.APIServerKeyName,

	certs.APIServerEtcdClientCertName: certs.APIServerEtcdClientCertName,
	certs.APIServerEtcdClientKeyName:  certs.APIServerEtcdClientKeyName,

	certs.APIServerKubeletClientCertName: certs.APIServerKubeletClientCertName,
	certs.APIServerKubeletClientKeyName:  certs.APIServerKubeletClientKeyName,

	certs.CACertName: certs.CACertName,
	certs.CAKeyName:  certs.CAKeyName,

	certs.FrontProxyCACertName: certs.FrontProxyCACertName,
	certs.FrontProxyCAKeyName:  certs.FrontProxyCAKeyName,

	certs.FrontProxyClientCertName: certs.FrontProxyClientCertName,
	certs.FrontProxyClientKeyName:  certs.FrontProxyClientKeyName,

	certs.ServiceAccountPrivateKeyName: certs.ServiceAccountPrivateKeyName,
	certs.ServiceAccountPublicKeyName:  certs.ServiceAccountPublicKeyName,

	certs.EtcdCACertName: strings.Replace(certs.EtcdCACertName, "/", "-", -1),
	certs.EtcdCAKeyName:  strings.Replace(certs.EtcdCAKeyName, "/", "-", -1),

	certs.EtcdHealthcheckClientCertName: strings.Replace(certs.EtcdHealthcheckClientCertName, "/", "-", -1),
	certs.EtcdHealthcheckClientKeyName:  strings.Replace(certs.EtcdHealthcheckClientKeyName, "/", "-", -1),

	certs.EtcdPeerCertName: strings.Replace(certs.EtcdPeerCertName, "/", "-", -1),
	certs.EtcdPeerKeyName:  strings.Replace(certs.EtcdPeerKeyName, "/", "-", -1),

	certs.EtcdServerCertName: strings.Replace(certs.EtcdServerCertName, "/", "-", -1),
	certs.EtcdServerKeyName:  strings.Replace(certs.EtcdServerKeyName, "/", "-", -1),
}

func ExecuteCerts(options *CertsCmd) error {
	inClusterConfig := ctrl.GetConfigOrDie()
	kubeClient, err := kubernetes.NewForConfig(inClusterConfig)
	if err != nil {
		return err
	}

	// get current namespace
	if options.Namespace == "" {
		options.Namespace, err = clienthelper.CurrentNamespace()
		if err != nil {
			return err
		}
	}

	secretName := options.Prefix + "-certs"
	_, err = kubeClient.CoreV1().Secrets(options.Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err == nil {
		klog.Infof("Certs secret already exists, skip generation")
		return nil
	}

	cfg, err := certs.SetInitDynamicDefaults()
	if err != nil {
		return err
	}

	// generate etcd server and peer sans
	etcdService := options.Prefix + "-etcd"
	serverSans := []string{etcdService, etcdService + "." + options.Namespace, etcdService + "." + options.Namespace + ".svc"}
	for i := 0; i < options.EtcdReplicas; i++ {
		hostname := etcdService + "-" + strconv.Itoa(i)
		serverSans = append(serverSans, hostname, hostname+"."+etcdService+"-headless", hostname+"."+etcdService+"-headless"+"."+options.Namespace)
	}

	cfg.ClusterName = options.ClusterName
	cfg.NodeRegistration.Name = options.Prefix + "-api"
	cfg.Etcd.Local = &certs.LocalEtcd{
		ServerCertSANs: serverSans,
		PeerCertSANs:   serverSans,
	}
	cfg.Networking.ServiceSubnet = options.ServiceCIDR
	cfg.Networking.DNSDomain = options.ClusterDomain
	cfg.ControlPlaneEndpoint = options.Prefix + "-api"
	cfg.CertificatesDir = options.CertificateDir
	cfg.LocalAPIEndpoint.AdvertiseAddress = "0.0.0.0"
	cfg.LocalAPIEndpoint.BindPort = 443
	err = certs.CreatePKIAssets(cfg)
	if err != nil {
		return errors.Wrap(err, "create pki assets")
	}

	err = certs.CreateJoinControlPlaneKubeConfigFiles(cfg.CertificatesDir, cfg)
	if err != nil {
		return errors.Wrap(err, "create kube configs")
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: options.Namespace,
		},
		Data: map[string][]byte{},
	}

	for fromName, toName := range certMap {
		data, err := ioutil.ReadFile(filepath.Join(options.CertificateDir, fromName))
		if err != nil {
			return errors.Wrap(err, "read "+fromName)
		}

		secret.Data[toName] = data
	}

	// finally create the secret
	_, err = kubeClient.CoreV1().Secrets(options.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "create certs secret")
	}

	klog.Infof("Successfully created certs secret %s/%s", options.Namespace, secretName)
	return nil
}
